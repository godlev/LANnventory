import { getDeviceTypeOption } from "./deviceTypes";
import { deviceDisplayName } from "./deviceIdentity";
import type { HostEvent } from "./exports";

export type ActivityTone = "discovered" | "online" | "offline" | "known" | "unknown" | "type";

export function activityIcon(eventType: string): string {
  switch (eventType) {
    case "online":
      return "bi-check-circle-fill";
    case "offline":
      return "bi-x-circle-fill";
    case "known":
      return "bi-bookmark-check-fill";
    case "unknown":
      return "bi-question-circle-fill";
    case "device-type-changed":
      return "bi-tag-fill";
    case "owner-changed":
      return "bi-person-fill";
    case "location-changed":
      return "bi-geo-alt-fill";
    case "notes-changed":
      return "bi-card-text";
    case "tags-changed":
      return "bi-tags-fill";
    case "pinned-changed":
      return "bi-pin-angle-fill";
    case "discovered":
    default:
      return "bi-plus-circle-fill";
  }
}

export function activityTone(eventType: string): ActivityTone {
  switch (eventType) {
    case "online":
      return "online";
    case "offline":
      return "offline";
    case "known":
      return "known";
    case "unknown":
      return "unknown";
    case "device-type-changed":
    case "owner-changed":
    case "location-changed":
    case "notes-changed":
    case "tags-changed":
    case "pinned-changed":
      return "type";
    case "discovered":
    default:
      return "discovered";
  }
}

export function activityDescription(event: HostEvent): string {
  switch (event.EventType) {
    case "online":
      return "Came online";
    case "offline":
      return "Went offline";
    case "known":
      return "Marked as known";
    case "unknown":
      return "Marked as unknown";
    case "device-type-changed": {
      return "Type changed to " + activityDeviceTypeLabel(event.NewValue);
    }
    case "owner-changed":
      return "Owner changed";
    case "location-changed":
      return "Location changed";
    case "notes-changed":
      return "Notes updated";
    case "tags-changed":
      return "Tags changed";
    case "pinned-changed":
      return event.NewValue === "true" ? "Device pinned" : "Device unpinned";
    case "discovered":
    default:
      return "New device detected";
  }
}

export function activityDetails(event: HostEvent): string {
  switch (event.EventType) {
    case "device-type-changed":
      return "Type changed: " + activityDeviceTypeLabel(event.OldValue) + " \u2192 " + activityDeviceTypeLabel(event.NewValue);
    case "owner-changed":
      return "Owner: " + textEventValue(event.OldValue) + " \u2192 " + textEventValue(event.NewValue);
    case "location-changed":
      return "Location: " + textEventValue(event.OldValue) + " \u2192 " + textEventValue(event.NewValue);
    case "notes-changed":
      return "Notes: " + notesEventValue(event.OldValue) + " \u2192 " + notesEventValue(event.NewValue);
    case "tags-changed":
      return "Tags: " + tagsEventValue(event.OldValue) + " \u2192 " + tagsEventValue(event.NewValue);
    case "pinned-changed":
      return "Pinned: " + booleanEventValue(event.OldValue) + " \u2192 " + booleanEventValue(event.NewValue);
    case "discovered":
    case "known":
    case "unknown":
      return compactNetworkDetail(event);
    case "online":
    case "offline":
    default:
      return "";
  }
}

export function activityHostName(event: HostEvent): string {
  return deviceDisplayName(event);
}

export function activityDeviceIcon(event: HostEvent): string {
  return getDeviceTypeOption(event.DeviceType).icon;
}

export function activityDeviceTypeLabel(value: string | null | undefined): string {
  return getDeviceTypeOption(value).label;
}

export function activityCategoryLabel(eventType: string): string {
  switch (eventType) {
    case "online":
    case "offline":
      return "Connectivity";
    default:
      return "Device changes";
  }
}

export function activityEventLabel(eventType: string): string {
  switch (eventType) {
    case "online":
      return "Online";
    case "offline":
      return "Offline";
    case "known":
      return "Marked known";
    case "unknown":
      return "Marked unknown";
    case "device-type-changed":
      return "Device type changed";
    case "owner-changed":
      return "Owner changed";
    case "location-changed":
      return "Location changed";
    case "notes-changed":
      return "Notes updated";
    case "tags-changed":
      return "Tags changed";
    case "pinned-changed":
      return "Pinned changed";
    case "discovered":
    default:
      return "New device detected";
  }
}

export function activityDayLabel(value: string): string {
  const match = value.match(/^(\d{4})-(\d{2})-(\d{2})/);
  if (!match) {
    return "Unknown day";
  }

  const [, year, month, day] = match;
  const date = new Date(Number(year), Number(month) - 1, Number(day));
  return date.toLocaleDateString("en", {
    day: "2-digit",
    month: "short",
    year: "numeric",
  });
}

export function relativeActivityTime(value: HostEvent | string, nowMs = Date.now()): string {
  const date = typeof value === "string" ? parseActivityDate(value) : parseActivityDisplayDate(value);
  if (date === null) {
    return "";
  }

  const seconds = Math.max(0, Math.floor((nowMs - date.getTime()) / 1000));
  if (seconds < 60) {
    return "now";
  }

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return minutes + " min ago";
  }

  const hours = Math.floor(minutes / 60);
  if (hours < 24) {
    return hours + " h ago";
  }

  if (hours < 48) {
    return "yesterday";
  }

  return Math.floor(hours / 24) + " d ago";
}

function compactNetworkDetail(event: HostEvent): string {
  if (event.IP && event.Iface) {
    return event.IP + " / " + event.Iface;
  }

  return event.IP || event.Iface || "";
}

function textEventValue(value: string | null | undefined): string {
  const text = (value ?? "").trim();
  return text === "" ? "Not set" : text;
}

function notesEventValue(value: string | null | undefined): string {
  const text = (value ?? "").replace(/\s+/g, " ").trim();
  if (text === "") {
    return "Not set";
  }

  return text.length > 90 ? text.slice(0, 87).trimEnd() + "..." : text;
}

function tagsEventValue(value: string | null | undefined): string {
  const tags = parseTagsValue(value);
  if (tags.length === 0) {
    return "Not set";
  }

  const label = tags.join(", ");
  return label.length > 90 ? label.slice(0, 87).trimEnd() + "..." : label;
}

function parseTagsValue(value: string | null | undefined): string[] {
  const raw = (value ?? "").trim();
  if (raw === "") {
    return [];
  }

  try {
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) {
      return [];
    }

    return parsed
      .filter((item): item is string => typeof item === "string")
      .map((item) => item.trim())
      .filter(Boolean);
  } catch {
    return raw === "" ? [] : [raw];
  }
}

function booleanEventValue(value: string | null | undefined): string {
  return value === "true" ? "Yes" : "No";
}

function parseActivityDate(value: string): Date | null {
  const match = value.match(/^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2}):(\d{2})$/);
  if (!match) {
    return null;
  }

  const [, year, month, day, hour, minute, second] = match;
  return new Date(
    Number(year),
    Number(month) - 1,
    Number(day),
    Number(hour),
    Number(minute),
    Number(second),
  );
}

function parseActivityDisplayDate(event: HostEvent): Date | null {
  const dateUTC = event.DateUTC?.trim();
  if (dateUTC) {
    const parsed = parseActivityDateUTC(dateUTC);
    if (parsed !== null) {
      return parsed;
    }
  }

  return parseActivityDate(event.Date);
}

function parseActivityDateUTC(value: string): Date | null {
  const match = value.match(/^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})Z$/);
  if (!match) {
    return null;
  }

  const [, year, month, day, hour, minute, second] = match;
  const timestamp = Date.UTC(
    Number(year),
    Number(month) - 1,
    Number(day),
    Number(hour),
    Number(minute),
    Number(second),
  );
  const date = new Date(timestamp);

  if (
    date.getUTCFullYear() !== Number(year)
    || date.getUTCMonth() !== Number(month) - 1
    || date.getUTCDate() !== Number(day)
    || date.getUTCHours() !== Number(hour)
    || date.getUTCMinutes() !== Number(minute)
    || date.getUTCSeconds() !== Number(second)
  ) {
    return null;
  }

  return date;
}
