import { createSignal } from "solid-js";

import type { Host, HostEvent } from "./exports";

export type HomeDeviceDisplayMode = "name" | "ip" | "mac" | "name-ip" | "name-mac";

type DeviceIdentity = Pick<Host | HostEvent, "Name" | "IP" | "Mac">;

const homeDeviceDisplayStorageKey = "homeDeviceDisplay";
const defaultHomeDeviceDisplayMode: HomeDeviceDisplayMode = "name-ip";

export const homeDeviceDisplayOptions: { key: HomeDeviceDisplayMode; label: string }[] = [
  { key: "name", label: "Name" },
  { key: "ip", label: "IP" },
  { key: "mac", label: "MAC" },
  { key: "name-ip", label: "Name + IP" },
  { key: "name-mac", label: "Name + MAC" },
];

const validHomeDeviceDisplayModes = new Set(homeDeviceDisplayOptions.map((option) => option.key));

export const [homeDeviceDisplayMode, setHomeDeviceDisplayModeSignal] = createSignal(readHomeDeviceDisplayMode());

export function normalizeHomeDeviceDisplayMode(value: unknown): HomeDeviceDisplayMode {
  return validHomeDeviceDisplayModes.has(value as HomeDeviceDisplayMode)
    ? value as HomeDeviceDisplayMode
    : defaultHomeDeviceDisplayMode;
}

export function readHomeDeviceDisplayMode(): HomeDeviceDisplayMode {
  try {
    return normalizeHomeDeviceDisplayMode(localStorage.getItem(homeDeviceDisplayStorageKey));
  } catch {
    return defaultHomeDeviceDisplayMode;
  }
}

export function setHomeDeviceDisplayMode(value: string) {
  const normalized = normalizeHomeDeviceDisplayMode(value);
  setHomeDeviceDisplayModeSignal(normalized);

  try {
    localStorage.setItem(homeDeviceDisplayStorageKey, normalized);
  } catch {
    // The preference is optional; rendering should continue when browser storage is blocked.
  }
}

export function deviceDisplayName(device: DeviceIdentity): string {
  return clean(device.Name) || clean(device.IP) || clean(device.Mac) || "Unknown device";
}

export function homeDeviceDisplayLabel(device: DeviceIdentity, mode = homeDeviceDisplayMode()): string {
  const name = clean(device.Name);
  const ip = clean(device.IP);
  const mac = clean(device.Mac);

  switch (mode) {
    case "name":
      return name || ip || mac || "Unknown device";
    case "ip":
      return ip || name || mac || "Unknown device";
    case "mac":
      return mac || name || ip || "Unknown device";
    case "name-mac":
      return combinePreferred(name, mac, ip);
    case "name-ip":
    default:
      return combinePreferred(name, ip, mac);
  }
}

function combinePreferred(primary: string, secondary: string, fallback: string) {
  if (primary && secondary) {
    return primary + " · " + secondary;
  }

  return primary || secondary || fallback || "Unknown device";
}

function clean(value: string | null | undefined) {
  return (value ?? "").trim();
}
