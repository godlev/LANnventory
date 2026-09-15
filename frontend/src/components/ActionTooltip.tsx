import { createSignal, JSX, onCleanup, onMount, Show } from "solid-js";
import { Portal } from "solid-js/web";

type TooltipPlacement = "auto" | "top" | "bottom";

type ActionTooltipProps = {
  title: string;
  detail?: string;
  placement?: TooltipPlacement;
  children: JSX.Element;
};

type TooltipPosition = {
  left: number;
  top: number;
  placement: "top" | "bottom";
};

const longPressDelayMs = 650;
const longPressVisibleMs = 3200;
const movementTolerancePx = 12;

function ActionTooltip(props: ActionTooltipProps) {
  const [visible, setVisible] = createSignal(false);
  const [position, setPosition] = createSignal<TooltipPosition>({
    left: 0,
    top: 0,
    placement: "bottom",
  });

  let anchor: HTMLSpanElement | undefined;
  let longPressTimer: number | undefined;
  let autoHideTimer: number | undefined;
  let pointerStartX = 0;
  let pointerStartY = 0;
  let hovering = false;
  let focused = false;
  let longPressShown = false;
  let suppressNextClick = false;
  let supportsPointerEvents = false;

  const clearLongPressTimer = () => {
    if (longPressTimer !== undefined) {
      window.clearTimeout(longPressTimer);
      longPressTimer = undefined;
    }
  };

  const clearAutoHideTimer = () => {
    if (autoHideTimer !== undefined) {
      window.clearTimeout(autoHideTimer);
      autoHideTimer = undefined;
    }
  };

  const hideTooltip = () => {
    clearAutoHideTimer();
    longPressShown = false;
    setVisible(false);
  };

  const hideIfIdle = () => {
    if (!hovering && !focused && !longPressShown) {
      hideTooltip();
    }
  };

  const showTooltip = (persistAfterTouch = false) => {
    if (!anchor) {
      return;
    }

    clearAutoHideTimer();

    const rect = anchor.getBoundingClientRect();
    const viewportWidth = Math.max(document.documentElement.clientWidth, window.innerWidth || 0);
    const tooltipMaxWidth = Math.min(288, Math.max(160, viewportWidth - 24));
    const halfWidth = tooltipMaxWidth / 2;
    const center = rect.left + rect.width / 2;
    const minCenter = halfWidth + 12;
    const maxCenter = Math.max(minCenter, viewportWidth - halfWidth - 12);
    const left = Math.min(Math.max(center, minCenter), maxCenter);

    const requested = props.placement ?? "auto";
    const resolvedPlacement = requested === "auto"
      ? (rect.top >= 96 ? "top" : "bottom")
      : requested;

    setPosition({
      left,
      top: resolvedPlacement === "top" ? rect.top - 8 : rect.bottom + 8,
      placement: resolvedPlacement,
    });
    setVisible(true);

    if (persistAfterTouch) {
      autoHideTimer = window.setTimeout(() => {
        longPressShown = false;
        hideIfIdle();
      }, longPressVisibleMs);
    }
  };

  const startLongPress = (clientX: number, clientY: number) => {
    clearLongPressTimer();
    pointerStartX = clientX;
    pointerStartY = clientY;
    longPressShown = false;

    longPressTimer = window.setTimeout(() => {
      longPressTimer = undefined;
      longPressShown = true;
      suppressNextClick = true;
      showTooltip(true);
    }, longPressDelayMs);
  };

  const cancelLongPressForMovement = (clientX: number, clientY: number) => {
    if (
      Math.abs(clientX - pointerStartX) > movementTolerancePx
      || Math.abs(clientY - pointerStartY) > movementTolerancePx
    ) {
      clearLongPressTimer();
    }
  };

  onMount(() => {
    if (!anchor) {
      return;
    }

    supportsPointerEvents = "PointerEvent" in window;

    const handlePointerEnter = (event: PointerEvent) => {
      if (event.pointerType === "touch") {
        return;
      }
      hovering = true;
      showTooltip();
    };
    const handlePointerLeave = (event: PointerEvent) => {
      if (event.pointerType === "touch") {
        return;
      }
      hovering = false;
      hideIfIdle();
    };
    const handlePointerDown = (event: PointerEvent) => {
      if (event.pointerType !== "touch") {
        return;
      }
      startLongPress(event.clientX, event.clientY);
    };
    const handlePointerMove = (event: PointerEvent) => {
      if (event.pointerType !== "touch") {
        return;
      }
      cancelLongPressForMovement(event.clientX, event.clientY);
    };
    const handlePointerEnd = (event: PointerEvent) => {
      if (event.pointerType !== "touch") {
        return;
      }
      clearLongPressTimer();
    };

    const handleMouseEnter = () => {
      if (supportsPointerEvents) {
        return;
      }
      hovering = true;
      showTooltip();
    };
    const handleMouseLeave = () => {
      if (supportsPointerEvents) {
        return;
      }
      hovering = false;
      hideIfIdle();
    };
    const handleTouchStart = (event: TouchEvent) => {
      if (supportsPointerEvents || event.touches.length !== 1) {
        return;
      }
      const touch = event.touches[0];
      startLongPress(touch.clientX, touch.clientY);
    };
    const handleTouchMove = (event: TouchEvent) => {
      if (supportsPointerEvents || event.touches.length !== 1) {
        return;
      }
      const touch = event.touches[0];
      cancelLongPressForMovement(touch.clientX, touch.clientY);
    };
    const handleTouchEnd = () => {
      if (supportsPointerEvents) {
        return;
      }
      clearLongPressTimer();
    };

    const handleFocusIn = () => {
      focused = true;
      showTooltip();
    };
    const handleFocusOut = (event: FocusEvent) => {
      const nextTarget = event.relatedTarget;
      if (nextTarget instanceof Node && anchor?.contains(nextTarget)) {
        return;
      }
      focused = false;
      hideIfIdle();
    };
    const handleClickCapture = (event: MouseEvent) => {
      if (!suppressNextClick) {
        return;
      }
      suppressNextClick = false;
      event.preventDefault();
      event.stopPropagation();
    };
    const handleContextMenu = (event: MouseEvent) => {
      if (suppressNextClick || longPressShown) {
        event.preventDefault();
      }
    };
    const handleDocumentPointerDown = (event: Event) => {
      const target = event.target;
      if (target instanceof Node && anchor?.contains(target)) {
        return;
      }
      hideTooltip();
    };
    const handleViewportChange = () => hideTooltip();

    if (supportsPointerEvents) {
      anchor.addEventListener("pointerenter", handlePointerEnter);
      anchor.addEventListener("pointerleave", handlePointerLeave);
      anchor.addEventListener("pointerdown", handlePointerDown);
      anchor.addEventListener("pointermove", handlePointerMove);
      anchor.addEventListener("pointerup", handlePointerEnd);
      anchor.addEventListener("pointercancel", handlePointerEnd);
      document.addEventListener("pointerdown", handleDocumentPointerDown, true);
    } else {
      anchor.addEventListener("mouseenter", handleMouseEnter);
      anchor.addEventListener("mouseleave", handleMouseLeave);
      anchor.addEventListener("touchstart", handleTouchStart, { passive: true });
      anchor.addEventListener("touchmove", handleTouchMove, { passive: true });
      anchor.addEventListener("touchend", handleTouchEnd);
      anchor.addEventListener("touchcancel", handleTouchEnd);
      document.addEventListener("mousedown", handleDocumentPointerDown, true);
      document.addEventListener("touchstart", handleDocumentPointerDown, true);
    }

    anchor.addEventListener("focusin", handleFocusIn);
    anchor.addEventListener("focusout", handleFocusOut);
    anchor.addEventListener("click", handleClickCapture, true);
    anchor.addEventListener("contextmenu", handleContextMenu);
    window.addEventListener("resize", handleViewportChange);
    window.addEventListener("scroll", handleViewportChange, true);

    onCleanup(() => {
      clearLongPressTimer();
      clearAutoHideTimer();

      if (!anchor) {
        return;
      }

      if (supportsPointerEvents) {
        anchor.removeEventListener("pointerenter", handlePointerEnter);
        anchor.removeEventListener("pointerleave", handlePointerLeave);
        anchor.removeEventListener("pointerdown", handlePointerDown);
        anchor.removeEventListener("pointermove", handlePointerMove);
        anchor.removeEventListener("pointerup", handlePointerEnd);
        anchor.removeEventListener("pointercancel", handlePointerEnd);
        document.removeEventListener("pointerdown", handleDocumentPointerDown, true);
      } else {
        anchor.removeEventListener("mouseenter", handleMouseEnter);
        anchor.removeEventListener("mouseleave", handleMouseLeave);
        anchor.removeEventListener("touchstart", handleTouchStart);
        anchor.removeEventListener("touchmove", handleTouchMove);
        anchor.removeEventListener("touchend", handleTouchEnd);
        anchor.removeEventListener("touchcancel", handleTouchEnd);
        document.removeEventListener("mousedown", handleDocumentPointerDown, true);
        document.removeEventListener("touchstart", handleDocumentPointerDown, true);
      }

      anchor.removeEventListener("focusin", handleFocusIn);
      anchor.removeEventListener("focusout", handleFocusOut);
      anchor.removeEventListener("click", handleClickCapture, true);
      anchor.removeEventListener("contextmenu", handleContextMenu);
      window.removeEventListener("resize", handleViewportChange);
      window.removeEventListener("scroll", handleViewportChange, true);
    });
  });

  return (
    <>
      <span class="wyl-tooltip-anchor" ref={anchor}>
        {props.children}
      </span>
      <Show when={visible()}>
        <Portal>
          <div
            class={"wyl-tooltip wyl-tooltip-" + position().placement}
            role="tooltip"
            style={{
              left: position().left + "px",
              top: position().top + "px",
              transform: position().placement === "top"
                ? "translate(-50%, -100%)"
                : "translate(-50%, 0)",
            }}
          >
            <div class="wyl-tooltip-title">{props.title}</div>
            <Show when={props.detail}>
              <div class="wyl-tooltip-detail">{props.detail}</div>
            </Show>
          </div>
        </Portal>
      </Show>
    </>
  );
}

export default ActionTooltip;
