import { createSignal, onCleanup, Show } from "solid-js";

import {
  apiRefreshHostIdentificationNames,
  apiScanHostPort,
  type HostIdentification,
  type IdentificationSuggestion,
} from "../../functions/api";
import { getDeviceTypeOption } from "../../functions/deviceTypes";

type IdentificationCardProps = {
  identification: HostIdentification | null;
  loading: boolean;
  error: string;
  onClose?: () => void;
  onRetry?: () => void;
  onEvidenceChanged?: () => void | Promise<void>;
};

type IdentificationRunSummary = {
  openPorts: number[];
  completed: number;
  indeterminate: number;
  stopped: boolean;
  partialError: string;
};

const identificationPorts = [22, 80, 443, 445, 515, 554, 631, 8000, 8080, 8443, 8554, 9100];

function IdentificationCard(props: IdentificationCardProps) {
  const [running, setRunning] = createSignal(false);
  const [progress, setProgress] = createSignal("");
  const [resultMessage, setResultMessage] = createSignal("");
  let investigationController: AbortController | undefined;

  const actionAvailable = (key: string) =>
    props.identification?.actions.some((action) => action.key === key && action.available) ?? false;

  const cancelInvestigation = () => {
    investigationController?.abort();
    investigationController = undefined;
  };

  onCleanup(cancelInvestigation);

  const refreshEvidence = async () => {
    await props.onEvidenceChanged?.();
  };

  const handleIdentify = async () => {
    const identification = props.identification;
    if (!identification || identification.hostId < 1 || running()) {
      return;
    }

    cancelInvestigation();
    const controller = new AbortController();
    investigationController = controller;
    setRunning(true);
    setResultMessage("");
    setProgress("Checking local names…");

    const summary: IdentificationRunSummary = {
      openPorts: [],
      completed: 0,
      indeterminate: 0,
      stopped: false,
      partialError: "",
    };
    let evidenceChanged = false;

    try {
      if (identification.currentAddress) {
        try {
          await apiRefreshHostIdentificationNames(identification.hostId, controller.signal);
          evidenceChanged = true;
        } catch (error) {
          if (controller.signal.aborted) {
            throw error;
          }
          summary.partialError = apiErrorMessage(error, "Local name lookup could not complete.");
        }
      }

      if (!controller.signal.aborted && identification.currentAddress && actionAvailable("service-scan")) {
        for (const port of identificationPorts) {
          if (controller.signal.aborted) {
            break;
          }

          setProgress(
            "Checking common services… " +
              summary.completed +
              "/" +
              identificationPorts.length,
          );

          try {
            const scan = await apiScanHostPort(identification.hostId, port, controller.signal);
            summary.completed++;
            evidenceChanged = true;
            if (scan.open) {
              summary.openPorts.push(port);
            } else if (scan.state === "indeterminate") {
              summary.indeterminate++;
            }
          } catch (error) {
            if (controller.signal.aborted) {
              throw error;
            }
            summary.partialError = apiErrorMessage(error, "A service check could not complete.");
            break;
          }
        }
      }

      if (evidenceChanged) {
        setProgress("Summarizing what was found…");
        await refreshEvidence();
      }

      setResultMessage(buildIdentificationResult(props.identification ?? identification, summary));
    } catch (error) {
      if (controller.signal.aborted) {
        summary.stopped = true;
        if (evidenceChanged) {
          try {
            await refreshEvidence();
          } catch {
            // Keep the best already-known evidence if the final refresh also fails.
          }
        }
        setResultMessage(buildIdentificationResult(props.identification ?? identification, summary));
      } else {
        setResultMessage(
          "I couldn't complete the identification check. " +
            apiErrorMessage(error, "Please try again."),
        );
      }
    } finally {
      if (investigationController === controller) {
        investigationController = undefined;
      }
      setProgress("");
      setRunning(false);
    }
  };

  const handlePrimaryAction = () => {
    if (running()) {
      investigationController?.abort();
      return;
    }
    void handleIdentify();
  };

  const handlePanelKeyDown = (event: KeyboardEvent) => {
    if (event.key !== "Escape") {
      return;
    }

    event.preventDefault();
    event.stopPropagation();
    if (running()) {
      investigationController?.abort();
      return;
    }

    props.onClose?.();
  };

  return (
    <section
      class="host-identification-panel"
      aria-label="Help identify unknown device"
      aria-busy={props.loading || running()}
      onKeyDown={handlePanelKeyDown}
    >
      <div class="host-identification-header">
        <div class="host-identification-heading">
          <span class="host-identification-icon" aria-hidden="true">
            <i class="bi bi-search"></i>
          </span>
          <div>
            <div class="host-identification-title">Help identify</div>
            <div class="host-identification-subtitle">One guided check · nothing is saved automatically</div>
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

      <Show
        when={!props.loading}
        fallback={
          <div class="host-identification-loading" role="status">
            <i class="bi bi-hourglass-split" aria-hidden="true"></i>
            <span>Loading identification context…</span>
          </div>
        }
      >
        <Show
          when={!props.error}
          fallback={
            <div class="host-identification-simple">
              <button type="button" class="btn btn-sm wyl-button host-identification-primary-action" onClick={() => props.onRetry?.()}>
                <i class="bi bi-arrow-repeat" aria-hidden="true"></i>
                <span>Retry</span>
              </button>
              <div class="host-identification-result is-error" role="alert">
                <i class="bi bi-exclamation-triangle-fill" aria-hidden="true"></i>
                <span>{props.error}</span>
              </div>
            </div>
          }
        >
          <Show when={props.identification}>
            <div class="host-identification-simple">
              <button
                type="button"
                class="btn btn-sm wyl-button host-identification-primary-action"
                aria-label={running() ? "Stop device identification" : "Identify device"}
                title="Uses retained evidence, local hostname resolution and a fixed set of common TCP ports. Reverse DNS follows the configured resolver."
                onClick={handlePrimaryAction}
              >
                <i class={running() ? "bi bi-stop-circle" : "bi bi-search"} aria-hidden="true"></i>
                <span>{running() ? "Stop" : resultMessage() ? "Identify again" : "Identify device"}</span>
              </button>

              <Show when={running() || resultMessage()}>
                <div class="host-identification-result" role="status" aria-live="polite">
                  <i class={running() ? "bi bi-hourglass-split" : "bi bi-info-circle-fill"} aria-hidden="true"></i>
                  <span>{running() ? progress() : resultMessage()}</span>
                </div>
              </Show>
            </div>
          </Show>
        </Show>
      </Show>
    </section>
  );
}

function buildIdentificationResult(
  identification: HostIdentification,
  run: IdentificationRunSummary,
): string {
  const assessment = identification.assessment;
  const prefix = run.stopped ? "Stopped early. " : "";

  if (assessment.conflicts.length > 0) {
    return (
      prefix +
      "I couldn't identify this device reliably because the current clues conflict: " +
      assessment.conflicts[0] +
      "."
    );
  }

  const typeSuggestion = assessment.suggestedDeviceType;
  const nameSuggestion = assessment.suggestedName;
  const parts: string[] = [];

  if (typeSuggestion) {
    parts.push(
      "Likely " +
        getDeviceTypeOption(typeSuggestion.value).label +
        " (" +
        confidenceText(typeSuggestion) +
        ")",
    );
  }

  if (nameSuggestion) {
    parts.push(
      (typeSuggestion ? "name " : "Possible name ") +
        '"' +
        nameSuggestion.value +
        '" (' +
        confidenceText(nameSuggestion) +
        ")",
    );
  }

  if (parts.length > 0) {
    let message = prefix + parts.join(", ") + ".";
    const reason = typeSuggestion?.reasons[0] ?? nameSuggestion?.reasons[0];
    if (reason) {
      message += " Evidence: " + reason + ".";
    }
    if (run.openPorts.length > 0) {
      message += " Open: " + run.openPorts.map(serviceLabel).join(", ") + ".";
    }
    if (run.partialError) {
      message += " Some checks could not complete.";
    }
    return message;
  }

  let message = prefix + "I couldn't identify this device reliably.";
  if (run.openPorts.length > 0) {
    message +=
      " I found " +
      run.openPorts.map(serviceLabel).join(", ") +
      ", but no current hostname or device-specific evidence was strong enough to classify it.";
  } else if (run.completed > 0) {
    message += " None of the checked common services provided a useful device-specific clue.";
  } else {
    message += " There is not enough current retained evidence to suggest a name or device type.";
  }

  if (run.indeterminate > 0 || run.partialError) {
    message += " Some checks were inconclusive.";
  }

  const caution = identification.warnings.find((warning) => warning.severity === "caution");
  if (caution) {
    message += " Caution: " + caution.message;
  }

  return message;
}

function confidenceText(suggestion: IdentificationSuggestion): string {
  switch (suggestion.confidence) {
    case "high":
      return "high confidence";
    case "medium":
      return "medium confidence";
    case "low":
      return "low confidence";
    default:
      return "uncertain";
  }
}

function serviceLabel(port: number): string {
  switch (port) {
    case 22:
      return "SSH (TCP 22)";
    case 80:
      return "HTTP (TCP 80)";
    case 443:
      return "HTTPS (TCP 443)";
    case 445:
      return "SMB (TCP 445)";
    case 515:
      return "LPD printing (TCP 515)";
    case 554:
      return "RTSP (TCP 554)";
    case 631:
      return "IPP printing (TCP 631)";
    case 8000:
      return "TCP 8000";
    case 8080:
      return "HTTP alt (TCP 8080)";
    case 8443:
      return "HTTPS alt (TCP 8443)";
    case 8554:
      return "RTSP alt (TCP 8554)";
    case 9100:
      return "raw printing (TCP 9100)";
    default:
      return "TCP " + port;
  }
}

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

export default IdentificationCard;
