import { createSignal, For, onCleanup, Show } from "solid-js";

import {
  apiRefreshHostIdentificationNames,
  apiScanHostPort,
  type HostIdentification,
  type IdentificationSourceStatus,
  type IdentificationSuggestion,
} from "../../functions/api";
import { getDeviceTypeOption } from "../../functions/deviceTypes";

type IdentificationCardProps = {
  identification: HostIdentification | null;
  loading: boolean;
  error: string;
  onClose?: () => void;
  onRetry?: () => void;
  onUseSuggestion?: (draft: { name?: string; deviceType?: string }) => void;
  onEvidenceChanged?: () => void | Promise<void>;
  onOpenNetwork?: (target: "services" | "identity") => void;
};

function IdentificationCard(props: IdentificationCardProps) {
  const unavailableSources = () => props.identification?.sources.filter((source) => !source.available) ?? [];
  const hasWarnings = () => (props.identification?.warnings.length ?? 0) > 0;
  const hasConflicts = () => (props.identification?.assessment.conflicts.length ?? 0) > 0;
  const [investigationStatus, setInvestigationStatus] = createSignal("");
  const [investigationError, setInvestigationError] = createSignal("");
  const [runningAction, setRunningAction] = createSignal("");
  const [presetProgress, setPresetProgress] = createSignal("");
  const [presetOpenPorts, setPresetOpenPorts] = createSignal<number[]>([]);
  let investigationController: AbortController | undefined;

  const actionAvailable = (key: string) =>
    props.identification?.actions.some((action) => action.key === key && action.available) ?? false;

  const cancelInvestigation = () => {
    investigationController?.abort();
    investigationController = undefined;
  };

  onCleanup(() => {
    cancelInvestigation();
  });

  const refreshEvidence = async () => {
    await props.onEvidenceChanged?.();
  };

  const handleRefreshNames = async () => {
    const identification = props.identification;
    if (!identification || identification.hostId < 1 || runningAction()) {
      return;
    }

    cancelInvestigation();
    const controller = new AbortController();
    investigationController = controller;
    setRunningAction("names");
    setInvestigationStatus("");
    setInvestigationError("");
    setPresetProgress("");
    setPresetOpenPorts([]);

    try {
      const result = await apiRefreshHostIdentificationNames(identification.hostId, controller.signal);
      if (controller.signal.aborted) {
        return;
      }
      if (result.sources.length === 0) {
        setInvestigationStatus("Name refresh completed. No local hostname source returned a new value.");
      } else {
        const sources = result.sources.map((source) => sourceLabel(source.source)).join(", ");
        setInvestigationStatus("Name evidence refreshed from " + sources + ".");
      }
      await refreshEvidence();
    } catch (error) {
      if (controller.signal.aborted) {
        setInvestigationStatus("Name refresh stopped.");
      } else {
        setInvestigationError(apiErrorMessage(error, "Name refresh failed."));
      }
    } finally {
      if (investigationController === controller) {
        investigationController = undefined;
      }
      setRunningAction("");
    }
  };

  const handlePresetScan = async (preset: PortPreset) => {
    const identification = props.identification;
    if (!identification || identification.hostId < 1 || !actionAvailable("service-scan") || runningAction()) {
      return;
    }

    cancelInvestigation();
    const controller = new AbortController();
    investigationController = controller;
    setRunningAction("ports:" + preset.key);
    setInvestigationStatus("");
    setInvestigationError("");
    setPresetOpenPorts([]);

    const openPorts: number[] = [];
    let completed = 0;
    let indeterminate = 0;

    try {
      for (const port of preset.ports) {
        if (controller.signal.aborted) {
          break;
        }
        setPresetProgress("Checking TCP " + port + " · " + completed + "/" + preset.ports.length + " complete");
        const result = await apiScanHostPort(identification.hostId, port, controller.signal);
        completed++;
        if (result.open) {
          openPorts.push(port);
          setPresetOpenPorts([...openPorts]);
        } else if (result.state === "indeterminate") {
          indeterminate++;
        }
      }

      if (controller.signal.aborted) {
        setInvestigationStatus("Port check stopped after " + completed + " of " + preset.ports.length + " ports.");
      } else {
        const openSummary = openPorts.length > 0 ? " Open: " + openPorts.join(", ") + "." : " No open preset ports found.";
        const uncertainSummary = indeterminate > 0 ? " " + indeterminate + " probe(s) were indeterminate." : "";
        setInvestigationStatus(preset.label + " check completed." + openSummary + uncertainSummary);
      }

      if (completed > 0) {
        await refreshEvidence();
      }
    } catch (error) {
      if (controller.signal.aborted) {
        setInvestigationStatus("Port check stopped after " + completed + " of " + preset.ports.length + " ports.");
        if (completed > 0) {
          await refreshEvidence();
        }
      } else {
        setInvestigationError(apiErrorMessage(error, preset.label + " port check failed."));
      }
    } finally {
      setPresetProgress("");
      if (investigationController === controller) {
        investigationController = undefined;
      }
      setRunningAction("");
    }
  };

  const handleStopInvestigation = () => {
    if (!runningAction()) {
      return;
    }
    investigationController?.abort();
  };

  const handlePanelKeyDown = (event: KeyboardEvent) => {
    if (event.key !== "Escape") {
      return;
    }

    event.preventDefault();
    event.stopPropagation();
    if (runningAction()) {
      handleStopInvestigation();
      return;
    }

    props.onClose?.();
  };

  const stateTitle = () => {
    const state = props.identification?.assessment.state;
    switch (state) {
      case "conflict":
        return "Conflicting clues need review";
      case "suggested":
        return "Useful retained clues found";
      case "known":
        return "Device is already marked Known";
      default:
        return "More evidence may be needed";
    }
  };

  const stateDetail = () => {
    const assessment = props.identification?.assessment;
    if (!assessment) {
      return "";
    }
    if (assessment.state === "suggested") {
      return "LANnventory has current evidence that can help fill the managed identity. Review remains required before anything is saved.";
    }
    if (assessment.state === "conflict") {
      return "Current evidence does not agree strongly enough for a safe identification suggestion.";
    }
    return assessment.reasons[0] ?? "Review the retained observations and choose what to investigate next.";
  };

  return (
    <section
      class="host-identification-panel"
      aria-label="Help identify unknown device"
      aria-busy={props.loading || Boolean(runningAction())}
      onKeyDown={handlePanelKeyDown}
    >
      <div class="host-identification-header">
        <div class="host-identification-heading">
          <span class="host-identification-icon" aria-hidden="true">
            <i class="bi bi-search"></i>
          </span>
          <div>
            <div class="host-identification-title">Help identify</div>
            <div class="host-identification-subtitle">Retained evidence only · no probes run automatically</div>
          </div>
        </div>
        <button
          type="button"
          class="btn btn-sm wyl-button host-identification-close"
          aria-label="Close identification helper"
          title="Close identification helper"
          onClick={() => props.onClose?.()}
        >
          <i class="bi bi-x-lg" aria-hidden="true"></i>
        </button>
      </div>

      <Show when={!props.loading} fallback={
        <div class="host-identification-loading" role="status">
          <i class="bi bi-hourglass-split" aria-hidden="true"></i>
          <span>Loading retained identification evidence…</span>
        </div>
      }>
        <Show
          when={!props.error}
          fallback={
            <div class="host-identification-error" role="alert">
              <div>
                <i class="bi bi-exclamation-triangle-fill" aria-hidden="true"></i>
                <span>{props.error}</span>
              </div>
              <button type="button" class="btn btn-sm wyl-button" onClick={() => props.onRetry?.()}>
                Retry
              </button>
            </div>
          }
        >
          <Show when={props.identification}>
            {(identification) => (
              <div class="host-identification-body">
                <div class="host-identification-summary">
                  <div class="host-identification-summary-copy">
                    <div class="host-identification-state">{stateTitle()}</div>
                    <div class="host-identification-state-detail">{stateDetail()}</div>
                  </div>
                  <div class="host-identification-clue-count" aria-label={identification().assessment.clueCount + " current identification clues"}>
                    <strong>{identification().assessment.clueCount}</strong>
                    <span>current clues</span>
                  </div>
                </div>

                <Show when={identification().assessment.suggestedName || identification().assessment.suggestedDeviceType}>
                  <section class="host-identification-suggestions" aria-label="Identification suggestions">
                    <div class="host-identification-suggestions-heading">
                      <div>
                        <strong>Suggested managed values</strong>
                        <span>Review the evidence, then copy only the values you want into the editable draft.</span>
                      </div>
                      <Show when={identification().assessment.suggestedName && identification().assessment.suggestedDeviceType}>
                        <button
                          type="button"
                          class="btn btn-sm wyl-button"
                          onClick={() => props.onUseSuggestion?.({
                            name: identification().assessment.suggestedName?.value,
                            deviceType: identification().assessment.suggestedDeviceType?.value,
                          })}
                        >
                          Use both
                        </button>
                      </Show>
                    </div>

                    <div class="host-identification-suggestion-grid">
                      <Show when={identification().assessment.suggestedName}>
                        {(suggestion) => (
                          <SuggestionRow
                            label="Name"
                            suggestion={suggestion()}
                            displayValue={suggestion().value}
                            onUse={() => props.onUseSuggestion?.({ name: suggestion().value })}
                          ></SuggestionRow>
                        )}
                      </Show>
                      <Show when={identification().assessment.suggestedDeviceType}>
                        {(suggestion) => (
                          <SuggestionRow
                            label="Device type"
                            suggestion={suggestion()}
                            displayValue={getDeviceTypeOption(suggestion().value).label}
                            onUse={() => props.onUseSuggestion?.({ deviceType: suggestion().value })}
                          ></SuggestionRow>
                        )}
                      </Show>
                    </div>

                    <div class="host-identification-draft-note">
                      <i class="bi bi-pencil-square" aria-hidden="true"></i>
                      <span>Using a suggestion only fills the managed edit draft. Nothing is saved and the device stays Unknown until you explicitly save or change its Known state.</span>
                    </div>
                  </section>
                </Show>

                <section class="host-identification-toolkit" aria-label="Identification investigation actions">
                  <div class="host-identification-toolkit-heading">
                    <div>
                      <strong>Investigate</strong>
                      <span>Run only the bounded checks you choose. Nothing runs when this panel opens.</span>
                    </div>
                    <Show when={runningAction()}>
                      <button
                        type="button"
                        class="btn btn-sm wyl-button"
                        aria-label="Stop current identification investigation"
                        onClick={handleStopInvestigation}
                      >
                        <i class="bi bi-stop-circle" aria-hidden="true"></i>
                        <span>Stop</span>
                      </button>
                    </Show>
                  </div>

                  <div class="host-identification-toolkit-row">
                    <button
                      type="button"
                      class="btn btn-sm wyl-button"
                      disabled={Boolean(runningAction()) || !identification().currentAddress}
                      onClick={() => void handleRefreshNames()}
                    >
                      <i class="bi bi-arrow-repeat" aria-hidden="true"></i>
                      <span>{runningAction() === "names" ? "Refreshing names…" : "Refresh names"}</span>
                    </button>
                    <span class="host-identification-toolkit-help">
                      Reverse DNS, system resolver and Avahi only. Reverse DNS follows the configured resolver and may be forwarded by that resolver.
                    </span>
                  </div>

                  <div class="host-identification-presets">
                    <div class="host-identification-presets-label">Bounded TCP presets</div>
                    <div class="host-identification-preset-buttons">
                      <For each={identificationPortPresets}>
                        {(preset) => (
                          <button
                            type="button"
                            class="btn btn-sm wyl-button"
                            title={preset.detail + " · TCP " + preset.ports.join(", ")}
                            disabled={Boolean(runningAction()) || !actionAvailable("service-scan")}
                            onClick={() => void handlePresetScan(preset)}
                          >
                            <i class={preset.icon} aria-hidden="true"></i>
                            <span>{runningAction() === "ports:" + preset.key ? preset.label + "…" : preset.label}</span>
                            <small>{preset.ports.length}</small>
                          </button>
                        )}
                      </For>
                    </div>
                    <div class="host-identification-toolkit-help">
                      Presets run sequentially against the Host's current IP and store only definitive service results. Each individual probe keeps the existing 3-second upper bound.
                    </div>
                  </div>

                  <div class="host-identification-shortcuts">
                    <button type="button" class="btn btn-sm wyl-button" onClick={() => props.onOpenNetwork?.("services")}>
                      <i class="bi bi-hdd-network" aria-hidden="true"></i>
                      <span>Review services</span>
                    </button>
                    <button type="button" class="btn btn-sm wyl-button" onClick={() => props.onOpenNetwork?.("identity")}>
                      <i class="bi bi-clock-history" aria-hidden="true"></i>
                      <span>Identity history</span>
                    </button>
                  </div>

                  <Show when={presetProgress()}>
                    <div class="host-identification-investigation-progress" role="status">
                      <i class="bi bi-hourglass-split" aria-hidden="true"></i>
                      <span>{presetProgress()}</span>
                    </div>
                  </Show>
                  <Show when={presetOpenPorts().length > 0}>
                    <div class="host-identification-open-ports" aria-label="Open ports found by current preset">
                      <span>Open now:</span>
                      <For each={presetOpenPorts()}>{(port) => <strong>TCP {port}</strong>}</For>
                    </div>
                  </Show>
                  <Show when={investigationStatus()}>
                    <div class="host-identification-investigation-status" role="status">{investigationStatus()}</div>
                  </Show>
                  <Show when={investigationError()}>
                    <div class="host-identification-error host-identification-investigation-error" role="alert">
                      <div>
                        <i class="bi bi-exclamation-triangle-fill" aria-hidden="true"></i>
                        <span>{investigationError()}</span>
                      </div>
                    </div>
                  </Show>
                </section>

                <div class="host-identification-sources" aria-label="Identification evidence sources">
                  <For each={identification().sources}>
                    {(source) => <SourceBadge source={source}></SourceBadge>}
                  </For>
                </div>

                <Show when={unavailableSources().length > 0}>
                  <div class="host-identification-partial" role="status">
                    <i class="bi bi-info-circle-fill" aria-hidden="true"></i>
                    <span>
                      Some retained sources are unavailable. The assessment uses only the evidence that could be loaded.
                    </span>
                  </div>
                </Show>

                <Show when={hasConflicts()}>
                  <div class="host-identification-message-list is-conflict" aria-label="Identification conflicts">
                    <For each={identification().assessment.conflicts}>
                      {(message) => (
                        <div>
                          <i class="bi bi-exclamation-diamond-fill" aria-hidden="true"></i>
                          <span>{message}</span>
                        </div>
                      )}
                    </For>
                  </div>
                </Show>

                <Show when={hasWarnings()}>
                  <div class="host-identification-message-list" aria-label="Identification cautions">
                    <For each={identification().warnings}>
                      {(warning) => (
                        <div class={warning.severity === "caution" ? "is-caution" : ""}>
                          <i class={warning.severity === "caution" ? "bi bi-exclamation-triangle-fill" : "bi bi-info-circle-fill"} aria-hidden="true"></i>
                          <span>{warning.message}</span>
                        </div>
                      )}
                    </For>
                  </div>
                </Show>
              </div>
            )}
          </Show>
        </Show>
      </Show>
    </section>
  );
}

type PortPreset = {
  key: string;
  label: string;
  detail: string;
  icon: string;
  ports: number[];
};

const identificationPortPresets: PortPreset[] = [
  {
    key: "common",
    label: "Common",
    detail: "SSH, web, SMB, RTSP, IPP and common admin endpoints",
    icon: "bi bi-grid-3x3-gap",
    ports: [22, 80, 443, 445, 554, 631, 8080, 8443, 9100],
  },
  {
    key: "camera",
    label: "Camera / IoT",
    detail: "Common web and RTSP endpoints",
    icon: "bi bi-camera-video",
    ports: [80, 443, 554, 8000, 8080, 8554],
  },
  {
    key: "printer",
    label: "Printer",
    detail: "Common web, LPD, IPP and raw printing endpoints",
    icon: "bi bi-printer",
    ports: [80, 443, 515, 631, 9100],
  },
];

function apiErrorMessage(error: unknown, fallback: string) {
  if (!(error instanceof Error)) {
    return fallback;
  }
  const message = error.message.trim();
  if (!message) {
    return fallback;
  }
  try {
    const parsed = JSON.parse(message);
    if (typeof parsed?.error === "string" && parsed.error.trim()) {
      return parsed.error.trim();
    }
  } catch {
    // Keep non-JSON API errors as returned.
  }
  return message;
}

function sourceLabel(source: string) {
  switch (source) {
    case "reverse-dns":
      return "Reverse DNS";
    case "system-resolver":
      return "System resolver";
    case "mdns":
      return "mDNS / Avahi";
    default:
      return source;
  }
}

function SuggestionRow(props: {
  label: string;
  suggestion: IdentificationSuggestion;
  displayValue: string;
  onUse: () => void;
}) {
  return (
    <div class="host-identification-suggestion">
      <div class="host-identification-suggestion-main">
        <span class="host-identification-suggestion-label">{props.label}</span>
        <strong class="host-identification-suggestion-value">{props.displayValue}</strong>
        <span class={"host-identification-confidence is-" + props.suggestion.confidence}>
          {confidenceLabelText(props.suggestion.confidence)}
        </span>
      </div>
      <div class="host-identification-suggestion-provenance">
        <span>Source: {props.suggestion.source}</span>
        <Show when={props.suggestion.reasons[0]}>
          <span>{props.suggestion.reasons[0]}</span>
        </Show>
      </div>
      <button type="button" class="btn btn-sm wyl-button" onClick={props.onUse}>
        Use {props.label.toLowerCase()}
      </button>
    </div>
  );
}

function confidenceLabelText(confidence: IdentificationSuggestion["confidence"]) {
  switch (confidence) {
    case "high":
      return "High confidence";
    case "medium":
      return "Medium confidence";
    case "low":
      return "Low confidence";
    default:
      return "No confidence";
  }
}

function SourceBadge(props: { source: IdentificationSourceStatus }) {
  const label = () => {
    switch (props.source.source) {
      case "discovery":
        return "Discovery";
      case "services":
        return "Services";
      case "workloads":
        return "Workloads";
      case "address-history":
        return "Address history";
      default:
        return props.source.source;
    }
  };

  const detail = () => {
    if (!props.source.available) {
      return "Unavailable";
    }
    if (props.source.truncated) {
      return props.source.included + " of " + props.source.total;
    }
    return String(props.source.included);
  };

  return (
    <span
      class={"host-identification-source" + (!props.source.available ? " is-unavailable" : "")}
      title={props.source.message || (props.source.truncated ? "Response is bounded; additional retained records exist." : "")}
    >
      <span>{label()}</span>
      <strong>{detail()}</strong>
    </span>
  );
}

export default IdentificationCard;
