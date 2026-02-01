document.addEventListener("DOMContentLoaded", function () {
  let mobileNavbarVisible = false;

  let darkMode =
    localStorage.theme === "dark" ||
    (!("theme" in localStorage) &&
      window.matchMedia("(prefers-color-scheme: dark)").matches);

  function setMode(isDark) {
     if (isDark) {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
  }

  // Initialize
  setMode(darkMode);

  function handleSwitchDarkMode() {
    darkMode = !darkMode;
    localStorage.setItem("theme", darkMode ? "dark" : "light");
    setMode(darkMode);
  }

  const mobileNavbarButton = document.getElementById("mobileNavbarButton");
  const themeToggles = document.querySelectorAll(".theme-toggle-btn");

  if (mobileNavbarButton) {
      mobileNavbarButton.addEventListener("click", function () {
        mobileNavbarVisible = !mobileNavbarVisible;
        const navbarDefault = document.getElementById("navbar-default");
        if (navbarDefault) {
            navbarDefault.classList.toggle("hidden", !mobileNavbarVisible);
        }
      });
  }

  if (themeToggles) {
      themeToggles.forEach(btn => {
          btn.addEventListener("click", handleSwitchDarkMode);
      });
  }
});
