import { For, onCleanup, onMount, Show } from "solid-js";

type ReleaseNotesDialogProps = {
  open: boolean;
  version: string;
  summary: string;
  releaseUrl: string;
  onClose: () => void;
};

function ReleaseNotesDialog(props: ReleaseNotesDialogProps) {
  const lines = () => props.summary
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);

  onMount(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        props.onClose();
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    onCleanup(() => document.removeEventListener("keydown", handleKeyDown));
  });

  return (
    <Show when={props.open}>
      <div
        class="update-notes-backdrop"
        role="presentation"
        onClick={(event) => {
          if (event.target === event.currentTarget) {
            props.onClose();
          }
        }}
      >
        <section
          class="update-notes-dialog"
          role="dialog"
          aria-modal="true"
          aria-labelledby="update-notes-title"
        >
          <header class="update-notes-header">
            <div>
              <div class="update-notes-kicker">Release notes</div>
              <h2 id="update-notes-title">{props.version || "LANnventory update"}</h2>
            </div>
            <button
              type="button"
              class="btn btn-sm wyl-button update-notes-close"
              aria-label="Close release notes"
              title="Close"
              onClick={props.onClose}
            >
              <i class="bi bi-x-lg" aria-hidden="true"></i>
            </button>
          </header>

          <div class="update-notes-body">
            <Show
              when={lines().length > 0}
              fallback={<p class="update-notes-empty">A local summary is not available for this release.</p>}
            >
              <For each={lines()}>
                {(line) =>
                  line.startsWith("- ")
                    ? <div class="update-notes-bullet"><span aria-hidden="true">•</span><span>{line.slice(2)}</span></div>
                    : line.endsWith(":")
                      ? <h3 class="update-notes-section">{line.slice(0, -1)}</h3>
                      : <p class="update-notes-paragraph">{line}</p>
                }
              </For>
            </Show>
          </div>

          <footer class="update-notes-footer">
            <Show when={props.releaseUrl}>
              <a
                class="btn btn-sm wyl-button"
                href={props.releaseUrl}
                target="_blank"
                rel="noreferrer"
              >
                <i class="bi bi-github" aria-hidden="true"></i>
                <span>View full release on GitHub</span>
              </a>
            </Show>
            <button type="button" class="btn btn-sm wyl-button" onClick={props.onClose}>
              Close
            </button>
          </footer>
        </section>
      </div>
    </Show>
  );
}

export default ReleaseNotesDialog;
