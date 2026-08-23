import { createSignal } from "solid-js";
import { A, useLocation } from "@solidjs/router";
import { appConfig, setAppConfig } from "../functions/exports";
import { apiGetConfig } from "../functions/api";

function Header() {

  const [themePath, setThemePath] = createSignal('');
  const location = useLocation();
  const localThemePath = (theme: string) => "/assets/themes/"+theme+"/bootstrap.min.css";
  const navItems = [
    { label: "Home", href: "/" },
    { label: "Config", href: "/config" },
    { label: "History", href: "/history" },
  ];

  const currentPath = () => location.pathname.replace(/\/$/, "") || "/";
  const navClass = (href: string) => {
    const path = href.replace(/\/$/, "") || "/";
    return "nav-link wyl-nav-tab" + (currentPath() === path ? " is-active" : "");
  };
  
  const setCurrentTheme = async () => {
    setAppConfig(await apiGetConfig());

    const theme = appConfig().Theme?appConfig().Theme:"sand";
    const color = appConfig().Color?appConfig().Color:"dark";
    
    setThemePath(localThemePath(theme));

    document.documentElement.setAttribute("data-bs-theme", color);
    color === "dark"
      ? document.documentElement.style.setProperty('--transparent-light', '#ffffff15')
      : document.documentElement.style.setProperty('--transparent-light', '#00000015');
  }
  setCurrentTheme();

  return (
    <>
    <link rel="stylesheet" href={themePath()}></link> {/* theme */}
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
        </ul>
        <ul class="navbar-nav wyl-navbar-actions">
          <li class="nav-item">
            <a class="nav-link wyl-navbar-github" target="_blank" rel="noreferrer" href="https://github.com/aceberg/WatchYourLAN" title="Github"><i class="bi bi-github"></i></a>
          </li>
        </ul>
      </div>
    </nav>
    </>
  )
};

export default Header
