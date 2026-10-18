package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/relationships"
)

func relationshipAPIRequest(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" { req.Header.Set("Content-Type", "application/json") }
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func relationshipAPIPayload(kind relationships.Type, sourceID, targetID int) string {
	payload := relationships.ManualCreateInput{
		Type: kind,
		Source: relationships.Endpoint{EntityType: relationships.EntityHost, EntityID: uint64(sourceID)},
		Target: relationships.Endpoint{EntityType: relationships.EntityHost, EntityID: uint64(targetID)},
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
}

func TestRelationshipAPIManualCRUDAndSafety(t *testing.T) {
	router := setupTestRouter(t)
	a := seedHost(t, models.Host{Name:"host-a",Mac:"AA:BB:CC:38:A3:01"})
	b := seedHost(t, models.Host{Name:"host-b",Mac:"AA:BB:CC:38:A3:02"})
	registry := getPath(router, "/api/relationships/types")
	var definitions []relationships.Definition
	if registry.Code != 200 || json.Unmarshal(registry.Body.Bytes(),&definitions) != nil || len(definitions) != 5 {
		t.Fatalf("registry code=%d body=%s",registry.Code,registry.Body.String())
	}
	for _, item := range definitions {
		if (item.Type == relationships.TypeHosts || item.Type == relationships.TypeMatchesHost) && item.ManualAllowed {
			t.Fatalf("Proxmox-owned type incorrectly writable: %+v",item)
		}
	}
	payload := relationshipAPIPayload(relationships.TypeManages,a.ID,b.ID)
	created := relationshipAPIRequest(router,http.MethodPost,"/api/relationships",payload)
	if created.Code != http.StatusCreated {
		t.Fatalf("create=%d body=%s",created.Code,created.Body.String())
	}
	var saved RelationshipResponse
	if err := json.Unmarshal(created.Body.Bytes(),&saved); err != nil {t.Fatal(err)}
	if saved.ID == 0 || saved.Provenance != relationships.ProvenanceManual || saved.Type != relationships.TypeManages ||
		saved.Directionality != relationships.Directed || saved.SourceLabel != "Manages" || saved.InverseLabel != "Managed by" {
		t.Fatalf("saved=%+v",saved)
	}
	idPath := fmt.Sprintf("/api/relationships/%d",saved.ID)
	if created.Header().Get("Location") != idPath {t.Fatalf("Location=%q",created.Header().Get("Location"))}
	if r := getPath(router,idPath); r.Code != 200 {t.Fatalf("GET id %d %s",r.Code,r.Body.String())}
	if r := relationshipAPIRequest(router,http.MethodPost,"/api/relationships",payload); r.Code != http.StatusConflict {
		t.Fatalf("duplicate=%d body=%s",r.Code,r.Body.String())
	}
	if r := relationshipAPIRequest(router,http.MethodPost,"/api/relationships",relationshipAPIPayload(relationships.TypeManages,b.ID,a.ID)); r.Code != 201 {
		t.Fatalf("directed reverse=%d body=%s",r.Code,r.Body.String())
	}
	for _, id := range []int{a.ID,b.ID} {
		r := getPath(router,fmt.Sprintf("/api/host/%d/relationships",id))
		var rows []RelationshipResponse
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(),&rows) != nil || len(rows) != 2 {
			t.Fatalf("host=%d relationships=%d body=%s",id,r.Code,r.Body.String())
		}
	}
	r := relationshipAPIRequest(router,http.MethodPatch,idPath,`{"note":"  port 8  "}`)
	var updated RelationshipResponse
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(),&updated) != nil || updated.Note != "port 8" || updated.Source != saved.Source || updated.Target != saved.Target {
		t.Fatalf("PATCH=%d body=%s",r.Code,r.Body.String())
	}
	if r := relationshipAPIRequest(router,http.MethodDelete,idPath,""); r.Code != 204 {
		t.Fatalf("DELETE=%d body=%s",r.Code,r.Body.String())
	}
	if r := getPath(router,idPath); r.Code != 404 {t.Fatalf("GET after delete=%d",r.Code)}
	if r := relationshipAPIRequest(router,http.MethodDelete,idPath,""); r.Code != 404 {t.Fatalf("redelete=%d",r.Code)}
}

func TestRelationshipAPIRejectsSpoofingMalformedAndSourceOwnedWrites(t *testing.T) {
	router := setupTestRouter(t)
	a := seedHost(t,models.Host{Name:"host-a",Mac:"AA:BB:CC:38:A3:11"})
	b := seedHost(t,models.Host{Name:"host-b",Mac:"AA:BB:CC:38:A3:12"})
	okJSON := relationshipAPIPayload(relationships.TypeConnectedTo,a.ID,b.ID)
	tests := []struct {name,body string;want int}{
		{"invalid body","{",400},
		{"missing fields","{}",400},
		{"null","null",400},
		{"trailing",okJSON+"{}",400},
		{"spoof provenance",strings.TrimSuffix(okJSON,"}")+`,"provenance":"proxmox"}`,400},
		{"spoof sourceInstance",strings.TrimSuffix(okJSON,"}")+`,"sourceInstance":"router-1"}`,400},
		{"spoof nested source",strings.Replace(okJSON,`"entityId":`,`"unknown":1,"entityId":`,1),400},
		{"reserved hosts",relationshipAPIPayload(relationships.TypeHosts,a.ID,b.ID),400},
		{"reserved matches",relationshipAPIPayload(relationships.TypeMatchesHost,a.ID,b.ID),400},
		{"self",relationshipAPIPayload(relationships.TypeConnectedTo,a.ID,a.ID),400},
		{"invalid type",relationshipAPIPayload("parent_of",a.ID,b.ID),400},
		{"missing current Host",relationshipAPIPayload(relationships.TypeConnectedTo,a.ID,9999),404},
		{"oversized",strings.TrimSuffix(okJSON,"}")+`,"note":"`+strings.Repeat("a",20000)+`"}`,400},
	}
	for _,tt:=range tests {
		t.Run(tt.name,func(t *testing.T){
			r:=relationshipAPIRequest(router,http.MethodPost,"/api/relationships",tt.body)
			if r.Code!=tt.want {t.Fatalf("%s: got %d want %d body=%s",tt.name,r.Code,tt.want,r.Body.String())}
		})
	}
	r:=relationshipAPIRequest(router,http.MethodPost,"/api/relationships",okJSON)
	if r.Code!=201 {t.Fatalf("valid POST=%d body=%s",r.Code,r.Body.String())}
	var saved RelationshipResponse
	if err:=json.Unmarshal(r.Body.Bytes(),&saved);err!=nil{t.Fatal(err)}
	path:=fmt.Sprintf("/api/relationships/%d",saved.ID)
	for _,body:=range []string{`{}`,`{"note":null}`,`{"source":{"entityType":"host","entityId":1},"note":"x"}`,`{"note":"x"}{"note":"y"}`,`{"note":"`+strings.Repeat("x",501)+`"}`} {
		if r:=relationshipAPIRequest(router,http.MethodPatch,path,body);r.Code!=400 {
			t.Fatalf("PATCH %q status=%d body=%s",body,r.Code,r.Body.String())
		}
	}
	for _,path:=range []string{"/api/relationships/0","/api/relationships/-1","/api/relationships/abc","/api/host/0/relationships"} {
		if r:=getPath(router,path);r.Code!=400{t.Fatalf("GET invalid=%s status=%d",path,r.Code)}
	}
	if r:=getPath(router,"/api/host/9999/relationships");r.Code!=404{t.Fatalf("missing host status=%d",r.Code)}
	if r:=getPath(router,"/api/relationships/9999");r.Code!=404{t.Fatalf("missing edge status=%d",r.Code)}
}

func TestRelationshipAPITargetSearchCurrentOnlyLiteralAndBounded(t *testing.T) {
	router:=setupTestRouter(t)
	a:=seedHost(t,models.Host{Name:"test router",Mac:"AA:BB:CC:38:A3:21",IP:"10.4.1.1",DNS:"router.local"})
	seedHost(t,models.Host{Name:"test NAS",Mac:"AA:BB:CC:38:A3:22",IP:"10.4.1.5",DNS:"nas.local"})
	seedHost(t,models.Host{Name:"another device",Mac:"AA:BB:CC:38:A3:23"})
	for _,q:=range []string{"TEST","nas.local","10.4.1.5","38:A3:22"} {
		r:=getPath(router,"/api/relationships/targets?query="+q)
		var list []RelationshipTargetResponse
		if r.Code!=200 || json.Unmarshal(r.Body.Bytes(),&list)!=nil || len(list)==0 {
			t.Fatalf("search=%s status=%d body=%s",q,r.Code,r.Body.String())
		}
	}
	r:=getPath(router,fmt.Sprintf("/api/relationships/targets?query=test&excludeHostId=%d",a.ID))
	var list []RelationshipTargetResponse
	if r.Code!=200 || json.Unmarshal(r.Body.Bytes(),&list)!=nil {t.Fatalf("exclude response %d %s",r.Code,r.Body.String())}
	for _,entry:=range list{if entry.ID==a.ID{t.Fatal("excluded Host present")}}
	for _,q:=range []string{"%25%25","__"} {
		r=getPath(router,"/api/relationships/targets?query="+q)
		if r.Code!=200 || json.Unmarshal(r.Body.Bytes(),&list)!=nil || len(list)!=0 {
			t.Fatalf("wildcard=%s response=%d %s",q,r.Code,r.Body.String())
		}
	}
	for _,url:=range []string{"/api/relationships/targets?query=x","/api/relationships/targets","/api/relationships/targets?query=test&excludeHostId=-1"}{
		r=getPath(router,url)
		if r.Code!=400 {t.Fatalf("invalid search=%s got=%d",url,r.Code)}
	}
	if err:=gdb.UpdateWithError("history",models.Host{ID:99,Name:"retired historian",Mac:"AA:BB:CC:38:A3:99"});err!=nil{t.Fatal(err)}
	r=getPath(router,"/api/relationships/targets?query=historian")
	if r.Code!=200 || json.Unmarshal(r.Body.Bytes(),&list)!=nil || len(list)!=0 {
		t.Fatalf("history leaked in targets: %d %s",r.Code,r.Body.String())
	}
	for i:=0;i<25;i++ {
		seedHost(t,models.Host{Name:fmt.Sprintf("test tagged %d",i),Mac:fmt.Sprintf("AA:BC:CD:38:%02X:%02X",i/16,i%16)})
	}
	r=getPath(router,"/api/relationships/targets?query=tagged")
	if r.Code!=200 || json.Unmarshal(r.Body.Bytes(),&list)!=nil || len(list)!=relationshipSearchLimit {
		t.Fatalf("target limit=%d; body=%s",r.Code,r.Body.String())
	}
}
