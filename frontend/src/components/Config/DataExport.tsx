import { createSignal } from "solid-js";
import { apiDownloadBackup, apiDownloadInventoryCSV } from "../../functions/api";

type ExportKind = "backup" | "csv" | "";

function DataExport() {
  const [busy, setBusy] = createSignal<ExportKind>("");
  const [status, setStatus] = createSignal("");
  const [error, setError] = createSignal("");

  const runExport = async (kind: Exclude<ExportKind, "">, action: () => Promise<void>, success: string) => {
    setBusy(kind);
    setStatus("");
    setError("");

    try {
      await action();
      setStatus(success);
    } catch {
      setError("Export failed. Check that the LANnventory backend is reachable, then try again.");
    } finally {
      setBusy("");
    }
  };

  return (
    <div class="card wyl-panel config-panel">
      <div class="card-header">Data backup</div>
      <div class="card-body table-responsive">
        <table class="table table-borderless">
          <tbody>
            <tr>
              <td class="config-field-label config-field-label-top">Portable backup</td>
              <td class="config-field-value">
                <button
                  type="button"
                  class="btn btn-sm wyl-button config-secondary-action"
                  disabled={busy() !== ""}
                  onClick={() => runExport("backup", apiDownloadBackup, "Backup download started")}
                >
                  {busy() === "backup" ? "Preparing backup" : "Download backup"}
                </button>
                <div class="config-field-helper">Exports current devices, presence history and Events as a portable JSON file.</div>
              </td>
            </tr>
            <tr>
              <td class="config-field-label config-field-label-top">Inventory CSV</td>
              <td class="config-field-value">
                <button
                  type="button"
                  class="btn btn-sm wyl-button config-secondary-action"
                  disabled={busy() !== ""}
                  onClick={() => runExport("csv", apiDownloadInventoryCSV, "CSV export started")}
                >
                  {busy() === "csv" ? "Preparing CSV" : "Export CSV"}
                </button>
                <div class="config-field-helper">Exports the current device inventory only. Restore and import are not available yet.</div>
              </td>
            </tr>
            <tr>
              <td></td>
              <td class="config-action-cell">
                <span class={"config-save-status" + (error() ? " config-save-error" : "")} role="status">
                  {error() || status()}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  );
}

export default DataExport;
