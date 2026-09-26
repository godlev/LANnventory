import { For, Show } from "solid-js";
import type { UpdateProgress, UpdateProgressStage } from "../../functions/updateApi";

type Props = {
  progress: UpdateProgress;
  onClose?: () => void;
  onRetry?: () => void;
  retryEnabled?: boolean;
};

const stages: Array<{ key: UpdateProgressStage; label: string }> = [
  { key: "preparing", label: "Preparing" },
  { key: "downloading", label: "Downloading package" },
  { key: "verifying", label: "Verifying package" },
  { key: "backup", label: "Creating backup" },
  { key: "installing", label: "Installing package" },
  { key: "restarting", label: "Restarting LANnventory" },
  { key: "health-check", label: "Verifying health" },
  { key: "complete", label: "Complete" },
];

function UpdateProgressPanel(props: Props) {
  const currentIndex = () => Math.max(0, stages.findIndex((item) => item.key === props.progress.stage));
  const failedIndex = () => {
    const stage = (props.progress.failedStage || props.progress.stage) as UpdateProgressStage;
    return Math.max(0, stages.findIndex((item) => item.key === stage));
  };
  const statusClass = () =>
    props.progress.status === "complete"
      ? " is-complete"
      : props.progress.status === "failed"
        ? " is-failed"
        : " is-running";
  const headline = () =>
    props.progress.status === "complete"
      ? "Update complete"
      : props.progress.status === "failed"
        ? "Update failed"
        : "Updating LANnventory";

  const stageState = (index: number) => {
    if (props.progress.status === "complete") return "done";
    if (props.progress.status === "failed") {
      if (index < failedIndex()) return "done";
      if (index === failedIndex()) return "failed";
      return "pending";
    }
    if (index < currentIndex()) return "done";
    if (index === currentIndex()) return "active";
    return "pending";
  };

  const stageIcon = (state: string) => {
    if (state === "done") return "bi bi-check-circle-fill";
    if (state === "failed") return "bi bi-x-circle-fill";
    if (state === "active") return "bi bi-arrow-repeat";
    return "bi bi-circle";
  };

  return (
    <section class={"update-progress-panel" + statusClass()} aria-live="polite">
      <header class="update-progress-header">
        <div>
          <div class="update-progress-kicker">LANnventory update</div>
          <h3>{headline()}</h3>
          <div class="update-progress-version">
            <span>{props.progress.previousVersion || "Unknown"}</span>
            <i class="bi bi-arrow-right" aria-hidden="true"></i>
            <strong>{props.progress.targetVersion || "Unknown"}</strong>
          </div>
        </div>
        <span class="update-progress-state">
          <i
            class={
              props.progress.status === "complete"
                ? "bi bi-check-circle-fill"
                : props.progress.status === "failed"
                  ? "bi bi-exclamation-triangle-fill"
                  : "bi bi-arrow-repeat"
            }
            aria-hidden="true"
          ></i>
          {props.progress.status === "complete" ? "Healthy" : props.progress.status === "failed" ? "Needs attention" : "In progress"}
        </span>
      </header>

      <div class="update-progress-stages">
        <For each={stages}>{(stage, index) => {
          const state = () => stageState(index());
          return (
            <div class={"update-progress-stage is-" + state()}>
              <i class={stageIcon(state()) + (state() === "active" ? " update-progress-spin" : "")} aria-hidden="true"></i>
              <span>{stage.label}</span>
            </div>
          );
        }}</For>
      </div>

      <Show when={props.progress.status === "complete"}>
        <div class="update-progress-result is-success">
          <div><span>Previous version</span><strong>{props.progress.previousVersion || "Unknown"}</strong></div>
          <div><span>Installed version</span><strong>{props.progress.targetVersion || "Unknown"}</strong></div>
          <div><span>Recovery backup</span><strong>{props.progress.backupCreated ? "Created successfully" : "Not reported"}</strong></div>
          <div><span>LANnventory</span><strong>Healthy</strong></div>
          <Show when={props.progress.backupPath}>
            <div class="update-progress-path"><span>Backup path</span><code>{props.progress.backupPath}</code></div>
          </Show>
        </div>
      </Show>

      <Show when={props.progress.status === "failed"}>
        <div class="update-progress-result is-error" role="alert">
          <div class="update-progress-error">
            <i class="bi bi-exclamation-triangle-fill" aria-hidden="true"></i>
            <div>
              <strong>{props.progress.error || "The update did not complete."}</strong>
              <span>Failed stage: {failedStageLabel(props.progress.failedStage || props.progress.stage)}</span>
            </div>
          </div>
          <div><span>Recovery backup</span><strong>{props.progress.backupCreated ? "Available" : "Not created"}</strong></div>
          <div><span>Service recovery</span><strong>{props.progress.serviceRestored ? "Service is running" : "Not confirmed"}</strong></div>
          <Show when={props.progress.backupPath}>
            <div class="update-progress-path"><span>Backup path</span><code>{props.progress.backupPath}</code></div>
          </Show>
        </div>
      </Show>

      <Show when={props.progress.status !== "running"}>
        <footer class="update-progress-actions">
          <Show when={props.progress.status === "failed" && props.onRetry}>
            <button
              type="button"
              class="btn btn-sm wyl-button"
              disabled={!props.retryEnabled}
              onClick={() => props.onRetry?.()}
            >
              <i class="bi bi-arrow-clockwise" aria-hidden="true"></i>
              <span>Retry update</span>
            </button>
          </Show>
          <Show when={props.onClose}>
            <button type="button" class="btn btn-sm wyl-button" onClick={() => props.onClose?.()}>
              Close
            </button>
          </Show>
        </footer>
      </Show>
    </section>
  );
}

function failedStageLabel(stage: string) {
  return stages.find((item) => item.key === stage)?.label || stage || "Unknown";
}

export default UpdateProgressPanel;
