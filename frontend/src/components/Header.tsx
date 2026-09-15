import { createEffect, createSignal, onCleanup, onMount, Show } from "solid-js";
import { A, useLocation } from "@solidjs/router";
import { appConfig, pageContext } from "../functions/exports";
import { normalizeColorMode, refreshAppConfig, setColorMode } from "../functions/theme";
import { apiGetUpdateStatus, setSharedUpdateStatus, sharedUpdateStatus } from "../functions/updateApi";
import { confirmUpdate, startUpdateFlow } from "../functions/updateFlow";

const updateStatusPollMs = 30 * 60 * 1000;

function Header() {

  const [themeError, setThemeError] = createSignal(false);
  const [supportOpen, setSupportOpen] = createSignal(false);
  const [mobileNavOpen, setMobileNavOpen] = createSignal(false);
  const updateStatus = sharedUpdateStatus;
  const setUpdateStatus = setSharedUpdateStatus;
  const [updateOpen, setUpdateOpen] = createSignal(false);
  const [updateInstalling, setUpdateInstalling] = createSignal(false);
  const [updateMessage, setUpdateMessage] = createSignal("");
  const [updateError, setUpdateError] = createSignal("");
  const location = useLocation();
  let mobileNavButtonRef: HTMLButtonElement | undefined;
  let mobileNavPanelRef: HTMLDivElement | undefined;
  let supportButtonRef: HTMLButtonElement | undefined;
  let supportPanelRef: HTMLDivElement | undefined;
  let updateButtonRef: HTMLButtonElement | undefined;
  let updatePanelRef: HTMLDivElement | undefined;
  let updateInitialTimer: number | undefined;
  let updatePollTimer: number | undefined;
  let updateReconnectTimer: number | undefined;
  const navItems = [
    { label: "Home", href: "/" },
    { label: "Presence", href: "/history" },
    { label: "Events", href: "/activity" },
  ];

  const currentPath = () => location.pathname.replace(/\/$/, "") || "/";
  const hostNavLabel = () => {
    const hostName = pageContext().hostName.trim();
    return hostName ? "Host · " + hostName : "Host";
  };
  const showHostContext = () => currentPath().startsWith("/host/");
  const isActivePath = (href: string) => currentPath() === (href.replace(/\/$/, "") || "/");
  const navClass = (href: string) => {
    return "nav-link wyl-nav-tab" + (isActivePath(href) ? " is-active" : "");
  };
  const settingsUtilityClass = () => "nav-link wyl-navbar-utility wyl-navbar-settings" + (isActivePath("/config") ? " is-active" : "");
  const showUpdateIndicator = () => !!updateStatus()?.available;
  const activeSectionLabel = () => {
    if (showHostContext()) {
      return hostNavLabel();
    }
    if (isActivePath("/history")) {
      return "Presence";
    }
    if (isActivePath("/activity")) {
      return "Events";
    }
    if (isActivePath("/config")) {
      return "Settings";
    }
    return "Home";
  };

  const currentColor = () => normalizeColorMode(appConfig().Color);
  const nextColor = () => currentColor() === "dark" ? "light" : "dark";
  const colorToggleLabel = () => currentColor() === "dark"
    ? "Switch to light mode"
    : "Switch to dark mode";
  const colorToggleIcon = () => currentColor() === "dark" ? "bi bi-sun-fill" : "bi bi-moon-stars-fill";

  const handleColorToggle = async () => {
    setThemeError(false);

    try {
      await setColorMode(nextColor());
    } catch (error) {
      setThemeError(true);
      console.error("Failed to save color mode", error);
    }
  };

  const handleColorToggleKeyDown = (event: KeyboardEvent) => {
    if (event.key !== "Enter" && event.key !== " ") {
      return;
    }

    event.preventDefault();
    handleColorToggle();
  };

  const closeSupport = (returnFocus = false) => {
    setSupportOpen(false);

    if (returnFocus) {
      queueMicrotask(() => supportButtonRef?.focus());
    }
  };

  const closeMobileNav = (returnFocus = false) => {
    setMobileNavOpen(false);

    if (returnFocus) {
      queueMicrotask(() => mobileNavButtonRef?.focus());
    }
  };

  const handleSupportToggle = () => {
    setSupportOpen((open) => !open);
  };

  const handleMobileNavToggle = () => {
    setMobileNavOpen((open) => !open);
  };

  const closeUpdatePopover = (returnFocus = false) => {
    setUpdateOpen(false);

    if (returnFocus) {
      queueMicrotask(() => updateButtonRef?.focus());
    }
  };

  const loadUpdateStatus = async (refresh = false) => {
    try {
      setUpdateStatus(await apiGetUpdateStatus(refresh));
      setUpdateError("");
    } catch {
      setUpdateStatus(undefined);
    }
  };

  const handleUpdateToggle = () => {
    setUpdateOpen((open) => !open);
  };

  const handleHeaderUpdateNow = async () => {
    const status = updateStatus();
    if (!status?.available || !status.installSupported || updateInstalling()) {
      return;
    }
    if (!confirmUpdate(status)) {
      return;
    }

    if (updateReconnectTimer !== undefined) {
      window.clearTimeout(updateReconnectTimer);
    }
    updateReconnectTimer = await startUpdateFlow(status, {
      onStatus: setUpdateStatus,
      onMessage: setUpdateMessage,
      onError: setUpdateError,
      onInstalling: setUpdateInstalling,
    });
  };

  onMount(() => {
    refreshAppConfig().catch((error) => {
      setThemeError(true);
      console.error("Failed to load application config", error);
    });

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as Node;

      if (supportOpen() && !supportButtonRef?.contains(target) && !supportPanelRef?.contains(target)) {
        closeSupport();
      }

      if (updateOpen() && !updateButtonRef?.contains(target) && !updatePanelRef?.contains(target)) {
        closeUpdatePopover();
      }

      if (mobileNavOpen() && !mobileNavButtonRef?.contains(target) && !mobileNavPanelRef?.contains(target)) {
        closeMobileNav();
      }
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") {
        return;
      }

      if (supportOpen()) {
        event.preventDefault();
        closeSupport(true);
      }

      if (updateOpen()) {
        event.preventDefault();
        closeUpdatePopover(true);
      }

      if (mobileNavOpen()) {
        event.preventDefault();
        closeMobileNav(true);
      }
    };

    document.addEventListener("pointerdown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    updateInitialTimer = window.setTimeout(() => void loadUpdateStatus(false), 1000);
    updatePollTimer = window.setInterval(() => void loadUpdateStatus(false), updateStatusPollMs);

    onCleanup(() => {
      document.removeEventListener("pointerdown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
      if (updateInitialTimer !== undefined) {
        window.clearTimeout(updateInitialTimer);
      }
      if (updatePollTimer !== undefined) {
        window.clearInterval(updatePollTimer);
      }
      if (updateReconnectTimer !== undefined) {
        window.clearTimeout(updateReconnectTimer);
      }
    });
  });

  createEffect(() => {
    currentPath();
    setSupportOpen(false);
    setMobileNavOpen(false);
    setUpdateOpen(false);
  });

  return (
    <>
    <nav class="navbar navbar-expand-md navbar-dark wyl-navbar">
      <div class="container-lg">
        <a class="navbar-brand" href="/" title="LANnventory" aria-label="LANnventory home">
          <img src="/fs/public/lanventory-navbar.png" class="wyl-navbar-logo" alt="LANnventory"/>
        </a>
        <div class="wyl-mobile-nav">
          <button
            ref={mobileNavButtonRef}
            type="button"
            class="nav-link wyl-nav-tab wyl-mobile-nav-toggle"
            title="Open navigation"
            aria-label="Open navigation"
            aria-expanded={mobileNavOpen() ? "true" : "false"}
            aria-controls="mobile-primary-nav"
            onClick={handleMobileNavToggle}
          >
            <i class="bi bi-list" aria-hidden="true"></i>
            <span>{activeSectionLabel()}</span>
          </button>
          <Show when={mobileNavOpen()}>
            <div
              ref={mobileNavPanelRef}
              id="mobile-primary-nav"
              class="wyl-mobile-nav-menu"
              role="menu"
            >
              {navItems.map((item) =>
                <A
                  class={navClass(item.href)}
                  href={item.href}
                  role="menuitem"
                  title={item.label}
                  aria-current={isActivePath(item.href) ? "page" : undefined}
                  onClick={() => closeMobileNav()}
                >
                  {item.label}
                </A>
              )}
              <Show when={showHostContext()}>
                <A
                  class="nav-link wyl-nav-tab wyl-nav-context is-active"
                  href={currentPath()}
                  role="menuitem"
                  title={hostNavLabel()}
                  aria-current="page"
                  onClick={() => closeMobileNav()}
                >
                  {hostNavLabel()}
                </A>
              </Show>
            </div>
          </Show>
        </div>
        <ul class="navbar-nav wyl-nav-tabs wyl-nav-tabs-desktop me-auto">
          {navItems.map((item) =>
          <li class="nav-item">
            <A class={navClass(item.href)} href={item.href} title={item.label} aria-current={isActivePath(item.href) ? "page" : undefined}>{item.label}</A>
          </li>
          )}
          <Show when={showHostContext()}>
            <li class="nav-item">
              <A
                class="nav-link wyl-nav-tab wyl-nav-context is-active"
                href={currentPath()}
                title={hostNavLabel()}
                aria-current="page"
              >
                {hostNavLabel()}
              </A>
            </li>
          </Show>
        </ul>
        <ul class="navbar-nav wyl-navbar-actions">
          <Show when={showUpdateIndicator()}>
            <li class="nav-item wyl-update-item">
              <button
                ref={updateButtonRef}
                type="button"
                class={"nav-link wyl-navbar-utility wyl-update-toggle" + (updateOpen() ? " is-active" : "")}
                title="LANnventory update available"
                aria-label="LANnventory update available"
                aria-expanded={updateOpen() ? "true" : "false"}
                aria-controls="update-popover"
                onClick={handleUpdateToggle}
              >
                <i class="bi bi-arrow-up-circle-fill" aria-hidden="true"></i>
              </button>
              <Show when={updateOpen()}>
                <div
                  ref={updatePanelRef}
                  id="update-popover"
                  class="wyl-update-popover"
                  role="dialog"
                  aria-labelledby="update-popover-title"
                >
                  <div id="update-popover-title" class="wyl-update-title">Update available</div>
                  <dl class="wyl-update-details">
                    <div>
                      <dt>Installed</dt>
                      <dd>{updateStatus()?.currentVersion || "Unknown"}</dd>
                    </div>
                    <div>
                      <dt>Latest</dt>
                      <dd>{updateStatus()?.latestVersion || "Unknown"}</dd>
                    </div>
                    <div>
                      <dt>Channel</dt>
                      <dd>{updateStatus()?.channel === "stable" ? "Stable" : "Beta"}</dd>
                    </div>
                  </dl>
                  <Show when={updateMessage()}>
                    <div class="wyl-update-popover-message" role="status">{updateMessage()}</div>
                  </Show>
                  <Show when={updateError()}>
                    <div class="wyl-update-popover-error" role="alert">{updateError()}</div>
                  </Show>
                  <div class="wyl-update-popover-actions">
                    <Show
                      when={updateStatus()?.installSupported}
                      fallback={<span class="wyl-update-popover-note">{updateStatus()?.installReason || "Automatic install is not supported on this system."}</span>}
                    >
                      <button
                        type="button"
                        class="btn btn-sm wyl-button"
                        disabled={updateInstalling()}
                        onClick={() => void handleHeaderUpdateNow()}
                      >
                        <i class={updateInstalling() ? "bi bi-hourglass-split" : "bi bi-download"} aria-hidden="true"></i>
                        <span>{updateInstalling() ? "Updating" : "Update now"}</span>
                      </button>
                    </Show>
                    <A class="btn btn-sm wyl-button" href="/config#updates" onClick={() => closeUpdatePopover()}>
                      <i class="bi bi-card-text" aria-hidden="true"></i>
                      <span>Review update</span>
                    </A>
                  </div>
                </div>
              </Show>
            </li>
          </Show>
          <li class="nav-item">
            <A
              class={settingsUtilityClass()}
              href="/config"
              title="Settings"
              aria-label="Settings"
              aria-current={isActivePath("/config") ? "page" : undefined}
            >
              <i class="bi bi-gear-fill" aria-hidden="true"></i>
            </A>
          </li>
          <li class="nav-item">
            <button
              type="button"
              class={"nav-link wyl-navbar-utility wyl-theme-toggle" + (themeError() ? " has-error" : "")}
              title={themeError() ? "Color mode could not be saved" : colorToggleLabel()}
              aria-label={themeError() ? "Color mode could not be saved" : colorToggleLabel()}
              onClick={handleColorToggle}
              onKeyDown={handleColorToggleKeyDown}
            >
              <i class={themeError() ? "bi bi-exclamation-triangle-fill" : colorToggleIcon()} aria-hidden="true"></i>
            </button>
          </li>
          <li class="nav-item wyl-support-item">
            <button
              ref={supportButtonRef}
              type="button"
              class={"nav-link wyl-navbar-utility wyl-support-toggle" + (supportOpen() ? " is-active" : "")}
              title="Support LANnventory"
              aria-label="Support LANnventory"
              aria-expanded={supportOpen() ? "true" : "false"}
              aria-controls="support-popover"
              onClick={handleSupportToggle}
            >
              <i class="bi bi-heart-fill" aria-hidden="true"></i>
            </button>
            <Show when={supportOpen()}>
              <div
                ref={supportPanelRef}
                id="support-popover"
                class="wyl-support-popover"
                role="dialog"
                aria-labelledby="support-popover-title"
              >
                <div id="support-popover-title" class="wyl-support-title">Support LANnventory</div>
                <p>LANnventory is an independent project originally based on WatchYourLAN and currently in active development. If you find the project useful and want to support its continued development, you can contribute via Revolut.</p>
                <a class="wyl-support-action" href="https://revolut.me/mirgeo" target="_blank" rel="noreferrer">
                  <i class="bi bi-heart-fill" aria-hidden="true"></i>
                  <span>SUPPORT VIA REVOLUT</span>
                </a>
                <p class="wyl-support-footer">Originally based on <a href="https://github.com/aceberg/WatchYourLAN" target="_blank" rel="noreferrer">WatchYourLAN by aceberg</a>.</p>
              </div>
            </Show>
          </li>
          <li class="nav-item">
            <a class="nav-link wyl-navbar-utility wyl-navbar-github" target="_blank" rel="noreferrer" href="https://github.com/godlev/LANnventory" title="LANnventory on GitHub" aria-label="LANnventory on GitHub"><i class="bi bi-github"></i></a>
          </li>
        </ul>
      </div>
    </nav>
    </>
  )
};

export default Header
