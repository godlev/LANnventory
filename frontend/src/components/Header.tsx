import { createSignal, onMount, Show } from "solid-js";
import { A, useLocation } from "@solidjs/router";
import { appConfig, pageContext } from "../functions/exports";
import { normalizeColorMode, refreshAppConfig, setColorMode } from "../functions/theme";

function Header() {

  const [themeError, setThemeError] = createSignal(false);
  const location = useLocation();
  const navItems = [
    { label: "Home", href: "/" },
    { label: "Config", href: "/config" },
    { label: "History", href: "/history" },
  ];

  const currentPath = () => location.pathname.replace(/\/$/, "") || "/";
  const hostNavLabel = () => {
    const hostName = pageContext().hostName.trim();
    return hostName ? "Host · " + hostName : "Host";
  };
  const showHostContext = () => currentPath().startsWith("/host/");
  const navClass = (href: string) => {
    const path = href.replace(/\/$/, "") || "/";
    return "nav-link wyl-nav-tab" + (currentPath() === path ? " is-active" : "");
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

  onMount(() => {
    refreshAppConfig().catch((error) => {
      setThemeError(true);
      console.error("Failed to load application config", error);
    });
  });

  return (
    <>
    <nav class="navbar navbar-expand-md navbar-dark wyl-navbar">
      <div class="container-lg">
        <a class="navbar-brand" href="/">
          <img src="/fs/public/favicon.png" class="wyl-navbar-logo"/>
        </a>
        <ul class="navbar-nav wyl-nav-tabs me-auto">
          {navItems.map((item) =>
          <li class="nav-item">
            <A class={navClass(item.href)} href={item.href} title={item.label} aria-current={navClass(item.href).includes("is-active") ? "page" : undefined}>{item.label}</A>
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
          <li class="nav-item">
            <a class="nav-link wyl-navbar-utility wyl-navbar-github" target="_blank" rel="noreferrer" href="https://github.com/aceberg/WatchYourLAN" title="Github"><i class="bi bi-github"></i></a>
          </li>
        </ul>
      </div>
    </nav>
    </>
  )
};

export default Header
