document.addEventListener("DOMContentLoaded", function () {
  let mobileNavbarVisible = false;

  const lightModeSVG = document.getElementById("svg-light-mode");
  const darkModeSVG = document.getElementById("svg-dark-mode");

  let darkMode =
    localStorage.theme === "dark" ||
    (!("theme" in localStorage) &&
      window.matchMedia("(prefers-color-scheme: dark)").matches);

  if (darkMode) {
    document.documentElement.classList.add("dark");
    lightModeSVG.style.display = "block";
    darkModeSVG.style.display = "none";
  } else {
    document.documentElement.classList.remove("dark");
    lightModeSVG.style.display = "none";
    darkModeSVG.style.display = "block";
  }

  function handleSwitchDarkMode() {
    darkMode = !darkMode;
    localStorage.setItem("theme", darkMode ? "dark" : "light");
    document.documentElement.classList.toggle("dark", darkMode);
    toggleSVGs();
  }

  const mobileNavbarButton = document.getElementById("mobileNavbarButton");
  const themeToggle = document.getElementById("theme-toggle");

  function toggleSVGs() {
    lightModeSVG.style.display =
      lightModeSVG.style.display === "none" ? "block" : "none";
    darkModeSVG.style.display =
      darkModeSVG.style.display === "none" ? "block" : "none";
  }
  mobileNavbarButton.addEventListener("click", function () {
    mobileNavbarVisible = !mobileNavbarVisible;
    const navbarDefault = document.getElementById("navbar-default");
    navbarDefault.classList.toggle("hidden", !mobileNavbarVisible);
  });

  themeToggle.addEventListener("click", handleSwitchDarkMode);
});
