import { For, Show } from "solid-js";
import { A } from "@solidjs/router";

import {
  activityDescription,
  activityDeviceIcon,
  activityHostName,
  activityIcon,
  activityTone,
  relativeActivityTime,
} from "../functions/activity";
import { deviceTypeTitle } from "../functions/deviceTypes";
import { homeDeviceDisplayLabel, type HomeDeviceDisplayMode } from "../functions/deviceIdentity";
import type { HostEvent } from "../functions/exports";

type ActivityFeedProps = {
  events: HostEvent[];
  emptyText: string;
  hostExists?: (event: HostEvent) => boolean;
  deviceDisplayMode?: HomeDeviceDisplayMode;
};

function ActivityFeed(props: ActivityFeedProps) {
  const canLinkHost = (event: HostEvent) => event.HostID > 0 && (props.hostExists?.(event) ?? false);
  const hostLabel = (event: HostEvent) => props.deviceDisplayMode
    ? homeDeviceDisplayLabel(event, props.deviceDisplayMode)
    : activityHostName(event);

  return (
    <Show
      when={props.events.length > 0}
      fallback={<div class="activity-empty">{props.emptyText}</div>}
    >
      <div class="activity-feed" role="list">
        <For each={props.events}>{(event) =>
          <div class={"activity-row activity-row-" + activityTone(event.EventType)} role="listitem">
            <span class="activity-event-icon" aria-hidden="true">
              <i class={"bi " + activityIcon(event.EventType)}></i>
            </span>
            <span
              class="activity-host-icon"
              title={deviceTypeTitle(event.DeviceType)}
              aria-label={deviceTypeTitle(event.DeviceType)}
              role="img"
            >
              <i class={"bi " + activityDeviceIcon(event)}></i>
            </span>
            <span class="activity-main">
              <Show
                when={canLinkHost(event)}
                fallback={<span class="activity-host-name">{hostLabel(event)}</span>}
              >
                <A href={"/host/" + event.HostID} class="activity-host-link">{hostLabel(event)}</A>
              </Show>
              <span class="activity-description">{activityDescription(event)}</span>
            </span>
            <time class="activity-time" dateTime={event.Date} title={event.Date}>
              {relativeActivityTime(event.Date)}
            </time>
          </div>
        }</For>
      </div>
    </Show>
  );
}

export default ActivityFeed;
