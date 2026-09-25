import { createSignal, onMount, Show } from "solid-js";
import { setShow } from "../../functions/exports";
import MacHistory from "../MacHistory"

function HistCard(_props: any) {

  const [today, setToday] = createSignal('');
  const [expanded, setExpanded] = createSignal(false);

  onMount(() => {
    setShow(15000);
    setToday(new Date().toLocaleDateString("en-CA"));
  });

  const handleDate = (date: string) => {
    setToday(date);
  };

  return (
    <details
      class="card wyl-panel host-history-panel host-history-disclosure"
      onToggle={(event) => setExpanded(event.currentTarget.open)}
    >
      <summary class="card-header host-history-header">
        <div>
          <div class="host-panel-title">Presence history</div>
          <div class="host-panel-subtitle">Historical online/offline presence for {_props.mac || "this device"}</div>
        </div>
        <span class="host-detail-section-badge">History</span>
      </summary>

      <Show when={expanded()}>
        <div class="card-body host-history-body">
          <div class="host-history-controls">
            <label class="host-history-date-control">
              <span>Presence date</span>
              <input
                type="date"
                class="form-control form-control-sm wyl-control host-date-input"
                value={today()}
                onInput={(e) => handleDate(e.currentTarget.value)}
              />
            </label>
          </div>

          {_props.mac !== "" && today() !== ""
          ? <MacHistory mac={_props.mac} date={today()}></MacHistory>
          : <span class="host-loading">Loading...</span>
          }
        </div>
      </Show>
    </details>
  )
}

export default HistCard
