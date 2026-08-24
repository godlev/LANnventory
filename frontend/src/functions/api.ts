import { isDeviceTypeValue, type DeviceTypeValue } from "./deviceTypes";
import type { ActivityDeviceOption, ActivityStats, Conf, Host, HostEvent } from "./exports";

export const apiPath = '';
export type ActivityCategory = "all" | "connectivity" | "changes";
export type ActivityEventType = "discovered" | "online" | "offline" | "known" | "unknown" | "device-type-changed";

type ActivityQuery = {
  category?: ActivityCategory;
  eventTypes?: ActivityEventType[];
  offset?: number;
  mac?: string;
  macs?: string[];
};

export const apiGetAllHosts = async () => {
  const url = apiPath+'/api/all';
  const hosts = await (await fetch(url)).json();

  return hosts;
};

export const apiGetConfig = async () => {

  const url = apiPath+'/api/config';
  const res = await (await fetch(url)).json();

  return res;
};

export const apiGetVersion = async () => {

  const url = apiPath+'/api/version';
  const res = await (await fetch(url)).json();

  return res;
};

export const apiGetActivity = async (limit = 20, query: ActivityQuery = {}): Promise<HostEvent[]> => {
  const params = new URLSearchParams({ limit: String(limit) });
  if (query.category !== undefined) {
    params.set("category", query.category);
  }
  query.eventTypes?.forEach((eventType) => {
    params.append("eventType", eventType);
  });
  if (query.offset !== undefined) {
    params.set("offset", String(query.offset));
  }
  if (query.mac) {
    params.append("mac", query.mac);
  }
  query.macs?.forEach((mac) => {
    if (mac) {
      params.append("mac", mac);
    }
  });

  const url = apiPath+'/api/activity?'+params.toString();
  const events = await (await fetch(url)).json();

  return events;
};

export const apiGetActivityStats = async (query: Pick<ActivityQuery, "mac" | "macs"> = {}): Promise<ActivityStats> => {
  const params = new URLSearchParams();
  if (query.mac) {
    params.append("mac", query.mac);
  }
  query.macs?.forEach((mac) => {
    if (mac) {
      params.append("mac", mac);
    }
  });

  const suffix = params.toString();
  const url = apiPath+'/api/activity/stats'+(suffix === "" ? "" : "?"+suffix);
  const stats = await (await fetch(url)).json();

  return stats;
};

export const apiGetActivityDevices = async (): Promise<ActivityDeviceOption[]> => {
  const url = apiPath+'/api/activity/devices';
  const devices = await (await fetch(url)).json();

  return devices;
};

export const apiGetHostActivity = async (id: number | string, limit = 10): Promise<HostEvent[]> => {
  const url = apiPath+'/api/host/'+id+'/activity?limit='+limit;
  const events = await (await fetch(url)).json();

  return events;
};

export const apiSetConfigColor = async (color: "dark" | "light"): Promise<Conf> => {
  const url = apiPath+'/api/config/color';
  const response = await fetch(url, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ color }),
  });

  if (!response.ok) {
    throw new Error(await response.text());
  }

  return await response.json();
};

export const apiSetRetention = async (presenceRetention: number, connectivityRetention: number): Promise<Conf> => {
  const url = apiPath+'/api/config/retention';
  const response = await fetch(url, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ presenceRetention, connectivityRetention }),
  });

  if (!response.ok) {
    throw new Error(await response.text());
  }

  return await response.json();
};

export const apiTestNotify = async () => {

  const url = apiPath+'/api/notify_test';
  await fetch(url);
};

export const apiEditHost = async (id:number, name:string, known:string) => {

  const url = apiPath+'/api/edit/'+id+'/'+name+'/'+known;
  const res = await (await fetch(url)).json();

  return res;
};

export const apiSetDeviceType = async (id: number, deviceType: DeviceTypeValue): Promise<Host> => {
  if (!isDeviceTypeValue(deviceType)) {
    throw new Error("invalid device type");
  }

  const url = apiPath+'/api/host/'+id+'/type';
  const response = await fetch(url, {
    method: 'PATCH',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ deviceType }),
  });

  if (!response.ok) {
    throw new Error(await response.text());
  }

  return await response.json();
};

export const apiGetHost = async (id:string) => {

  const url = apiPath+'/api/host/'+id;
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(await response.text());
  }

  const res = await response.json();

  return res;
};

export const apiDelHost = async (id:number) => {

  const url = apiPath+'/api/host/del/'+id;
  const res = await (await fetch(url)).json();

  return res;
};

export const apiPortScan = async (ip:string, port:number) => {

  const url = apiPath+'/api/port/'+ip+'/'+port;
  const res = await (await fetch(url)).json();

  return res;
};

export const apiGetHistory = async (mac:string) => {
  const url = apiPath+'/api/history/'+mac+'/?num=210';
  const hosts = await (await fetch(url)).json();

  return hosts;
};

export const apiGetHistoryByDate = async (mac:string, date: string) => {
  const url = apiPath+'/api/history/'+mac+'/'+date;
  const hosts = await (await fetch(url)).json();

  return hosts;
};

export const apiWOL = async (mac:string) => {

  const url = apiPath+'/api/wol/'+mac;
  const res = await (await fetch(url)).json();

  return res;
};
