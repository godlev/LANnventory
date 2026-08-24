import { createSignal } from "solid-js";
import { createStore } from "solid-js/store";

export interface Host {
	ID:    number;
	Name:  string;
	DNS:   string;
	Iface: string;
	IP:    string;
	Mac:   string;
	Hw:    string;
	Date:  string;
	Known: number;
	Now:   number;
	DeviceType: string;
};

export interface HostEvent {
	ID:         number;
	HostID:     number;
	Mac:        string;
	Name:       string;
	EventType:  string;
	Date:       string;
	IP:         string;
	Iface:      string;
	DeviceType: string;
	OldValue:   string;
	NewValue:   string;
};

export interface Conf {
	Host:	   string;
	Port:	   string;
	Theme:	   string;
	Color:     string;
	DirPath:   string;
	Timeout:   number;
	NodePath:  string;
	LogLevel:  string;
	Ifaces:	   string;
	ArpArgs:   string;
	ArpStrs:   string[];
	TrimHist:  number;
	ConnectivityRetention: number;
	ShoutURL:  string;
	UseDB:     string;
	PGConnect: string;
	// InfluxDB
	InfluxEnable:  boolean;
	InfluxAddr:    string;
	InfluxToken:   string;
	InfluxOrg:     string;
	InfluxBucket:  string;
	InfluxSkipTLS: boolean;
	// Prometheus
	PrometheusEnable: boolean;
};

export type SortDirection = "ascending" | "descending";

export interface SortState {
	field: keyof Host | "";
	direction: SortDirection | "";
};

export interface FilterState {
	Iface: string;
	Known: number | "";
	Now: number | "";
	Search: string;
};

export interface PageContext {
	kind: "" | "host";
	hostName: string;
};

export const emptyHost:Host = {
	ID:    0,
	Name:  "",
	DNS:   "",
	Iface: "",
	IP:    "",
	Mac:   "",
	Hw:    "",
	Date:  "",
	Known: 0,
	Now:   0,
	DeviceType: "",
};

export const emptyConf:Conf = {
	Host:	 "",
	Port:	 "",
	Theme:	 "",
	Color:   "",
	DirPath: "",
	Timeout: 120,
	NodePath: "",
	LogLevel: "",
	Ifaces:	 "",
	ArpArgs: "",
	ArpStrs: [],
	TrimHist: 48,
	ConnectivityRetention: 48,
	ShoutURL: "",
	UseDB: "",
	PGConnect: "",
	InfluxEnable:  false,
	InfluxAddr:    "",
	InfluxToken:   "",
	InfluxOrg:     "",
	InfluxBucket:  "",
	InfluxSkipTLS: false,
	PrometheusEnable: false,
};

export const emptyFilterState:FilterState = {
	Iface: "",
	Known: "",
	Now: "",
	Search: "",
};

export const emptyPageContext:PageContext = {
	kind: "",
	hostName: "",
};

export const [allHosts, setAllHosts] = createStore<Host[]>([]);
export const [bkpHosts, setBkpHosts] = createSignal<Host[]>([]);

export const [ifaces, setIfaces] = createSignal<string[]>([]);
export const hasMultipleIfaces = () => ifaces().filter((iface) => iface.trim() !== "").length > 1;

export const [appConfig, setAppConfig] = createSignal<Conf>(emptyConf);

export const [filterState, setFilterState] = createSignal<FilterState>(emptyFilterState);
export const [pageContext, setPageContext] = createSignal<PageContext>(emptyPageContext);
export const [sortState, setSortState] = createSignal<SortState>({
	field: "",
	direction: "",
});

export const [editNames, setEditNames] = createSignal(false);

export const [show, setShow] = createSignal<number>(200);

export const [histUpdOnFilter, setHistUpdOnFilter] = createSignal(false);

export const [selectedIDs, setSelectedIDs] = createSignal<number[]>([]);
