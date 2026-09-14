import { createMemo, createSignal, For, onCleanup, Show } from "solid-js";

type InventoryAutocompleteProps = {
  id: string;
  label: string;
  value: string;
  options: string[];
  onInput: (value: string) => void;
};

const maxVisibleOptions = 8;

function InventoryAutocomplete(props: InventoryAutocompleteProps) {
  const [open, setOpen] = createSignal(false);
  const [activeIndex, setActiveIndex] = createSignal(-1);
  let rootRef: HTMLDivElement | undefined;
  let inputRef: HTMLInputElement | undefined;

  const listboxId = () => props.id + "-options";
  const optionId = (index: number) => props.id + "-option-" + index;
  const normalizedOptions = createMemo(() => {
    const seen = new Set<string>();
    return props.options
      .map((option) => option.trim())
      .filter((option) => {
        if (option === "") {
          return false;
        }
        const key = option.toLowerCase();
        if (seen.has(key)) {
          return false;
        }
        seen.add(key);
        return true;
      });
  });
  const visibleOptions = createMemo(() => {
    const query = props.value.trim().toLowerCase();
    const filtered = query === ""
      ? normalizedOptions()
      : normalizedOptions().filter((option) => option.toLowerCase().includes(query));

    return filtered.slice(0, maxVisibleOptions);
  });
  const activeDescendant = () => {
    const index = activeIndex();
    return open() && index >= 0 && index < visibleOptions().length ? optionId(index) : undefined;
  };

  const closeOptions = () => {
    setOpen(false);
    setActiveIndex(-1);
  };

  const selectOption = (option: string) => {
    props.onInput(option);
    closeOptions();
    queueMicrotask(() => inputRef?.focus());
  };

  const handleInput = (value: string) => {
    props.onInput(value);
    setOpen(true);
    setActiveIndex(-1);
  };

  const handleKeyDown = (event: KeyboardEvent) => {
    if (event.key === "Escape") {
      closeOptions();
      return;
    }

    const options = visibleOptions();
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setOpen(true);
      setActiveIndex((current) => Math.min(current + 1, options.length - 1));
      return;
    }
    if (event.key === "ArrowUp") {
      event.preventDefault();
      setOpen(true);
      setActiveIndex((current) => Math.max(current - 1, 0));
      return;
    }
    if (event.key === "Enter" && open()) {
      const index = activeIndex();
      if (index >= 0 && index < options.length) {
        event.preventDefault();
        selectOption(options[index]);
      }
    }
  };

  const handlePointerDown = (event: PointerEvent) => {
    if (!rootRef?.contains(event.target as Node)) {
      closeOptions();
    }
  };
  document.addEventListener("pointerdown", handlePointerDown);
  onCleanup(() => document.removeEventListener("pointerdown", handlePointerDown));

  return (
    <div
      ref={rootRef}
      class="inventory-autocomplete"
      role="combobox"
      aria-expanded={open() && visibleOptions().length > 0 ? "true" : "false"}
      aria-haspopup="listbox"
      aria-owns={listboxId()}
    >
      <input
        ref={inputRef}
        id={props.id}
        type="text"
        class="form-control form-control-sm wyl-control"
        value={props.value}
        aria-label={props.label}
        aria-autocomplete="list"
        aria-controls={listboxId()}
        aria-activedescendant={activeDescendant()}
        autocomplete="off"
        onFocus={() => setOpen(true)}
        onInput={(event) => handleInput(event.currentTarget.value)}
        onKeyDown={handleKeyDown}
      ></input>
      <Show when={open() && visibleOptions().length > 0}>
        <div id={listboxId()} class="inventory-autocomplete-list" role="listbox">
          <For each={visibleOptions()}>{(option, index) =>
            <button
              id={optionId(index())}
              type="button"
              class={"inventory-autocomplete-option" + (index() === activeIndex() ? " is-active" : "")}
              role="option"
              aria-selected={index() === activeIndex()}
              onPointerDown={(event) => event.preventDefault()}
              onClick={() => selectOption(option)}
            >
              {option}
            </button>
          }</For>
        </div>
      </Show>
    </div>
  );
}

export default InventoryAutocomplete;
