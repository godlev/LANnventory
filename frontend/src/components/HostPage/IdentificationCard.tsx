import { For, Show } from "solid-js";

import type { HostIdentification, IdentificationSourceStatus, IdentificationSuggestion } from "../../functions/api";
import { getDeviceTypeOption } from "../../functions/deviceTypes";

type IdentificationCardProps = {
  identification: HostIdentification | null;
  loading: boolean;
  error: string;
  onClose?: () => void;
  onRetry?: () => void;
  onUseSuggestion?: (draft: { name?: string; deviceType?: string }) => void;
};

function IdentificationCard(props: IdentificationCardProps) {
  const unavailableSources = () => props.identification?.sources.filter((source) => !source.available) ?? [];
  const hasWarnings = () => (props.identification?.warnings.length ?? 0) > 0;
  const hasConflicts = () => (props.identification?.assessment.conflicts.length ?? 0) > 0;

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
    <section class="host-identification-panel" aria-label="Help identify unknown device">
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
