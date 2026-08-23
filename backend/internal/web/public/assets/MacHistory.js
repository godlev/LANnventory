import{C as w,D as y,E as v,o as m,G as H,d as f,S as d,f as D,s as I,H as h,k as M,F as b,t as g}from"./index.js";async function u(a,r){let e=[];return r===""?e=await w(a):e=await y(a,r),e!=null?(e.sort((s,t)=>s.Date<t.Date?1:-1),e):[]}var x=g("<i>");function F(a){const[r,e]=v([]);let s;return m(async()=>{const t=await u(a.mac,a.date);e(t),s=setInterval(async()=>{const o=await u(a.mac,a.date);e(o)},6e4)}),H(()=>{clearInterval(s)}),f(b,{each:r,children:(t,o)=>f(d,{get when(){return o()<M()},get children(){var i=x();return D(n=>{var c="Date:"+t.Date+`
Iface:`+t.Iface+`
IP:`+t.IP+`
Known:`+t.Known,l=t.Now===0?"my-box-off":"my-box-on";return c!==n.e&&I(i,"title",n.e=c),l!==n.t&&h(i,n.t=l),n},{e:void 0,t:void 0}),i}})})}export{F as M};
