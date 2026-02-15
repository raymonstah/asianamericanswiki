document.addEventListener("DOMContentLoaded", function () {
  // --- Theme Toggle Logic ---
  let mobileNavbarVisible = false;
  let darkMode = localStorage.theme === "dark" || (!("theme" in localStorage) && window.matchMedia("(prefers-color-scheme: dark)").matches);

  function setMode(isDark) {
    if (isDark) {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
  }
  setMode(darkMode);

  function handleSwitchDarkMode() {
    darkMode = !darkMode;
    localStorage.setItem("theme", darkMode ? "dark" : "light");
    setMode(darkMode);
  }

  const mobileNavbarButton = document.getElementById("mobileNavbarButton");
  if (mobileNavbarButton) {
    mobileNavbarButton.addEventListener("click", function () {
      mobileNavbarVisible = !mobileNavbarVisible;
      const navbarDefault = document.getElementById("navbar-default");
      if (navbarDefault) {
        navbarDefault.classList.toggle("hidden", !mobileNavbarVisible);
      }
    });
  }

  const themeToggles = document.querySelectorAll(".theme-toggle-btn");
  if (themeToggles) {
    themeToggles.forEach(btn => btn.addEventListener("click", handleSwitchDarkMode));
  }

  // --- Search Logic ---
  const searchInput = document.querySelector('input[name="search"]');
  const searchResults = document.getElementById('search-results');

  if (searchInput && searchResults) {
    
    // Function to hide results
    const hideResults = () => {
      searchResults.innerHTML = '';
      selectedIndex = -1;
    };

    // Function to show results (trigger search)
    const showResults = () => {
      if (searchInput.value.trim() !== '' && searchResults.innerHTML.trim() === '') {
        // Dispatch custom 'search' event for HTMX
        searchInput.dispatchEvent(new Event('search'));
      }
    };

    // 1. Hide on Click Outside
    document.addEventListener('click', (e) => {
      const isClickInside = searchInput.contains(e.target) || searchResults.contains(e.target);
      if (!isClickInside) {
        hideResults();
      }
    });

    // 2. Hide on Escape
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
        hideResults();
        searchInput.blur();
      }
    });

    // 3. Show on Click (if has text)
    searchInput.addEventListener('click', showResults);
    
    // 4. Show on Focus (if has text - e.g. tabbing in)
    searchInput.addEventListener('focus', showResults);

    // --- Keyboard Navigation ---
    let selectedIndex = -1;

    function getInteractiveItems() {
      return searchResults.querySelectorAll('a, button');
    }

    function updateSelection() {
      const items = getInteractiveItems();
      items.forEach((item, index) => {
        if (index === selectedIndex) {
          item.classList.add('bg-[var(--color-background)]');
          item.scrollIntoView({ block: 'nearest' });
        } else {
          item.classList.remove('bg-[var(--color-background)]');
        }
      });
    }

    // Reset selection when results change
    document.addEventListener('htmx:afterSwap', (e) => {
      if (e.target.id === 'search-results') {
        selectedIndex = -1;
      }
    });

    searchInput.addEventListener('keydown', (e) => {
      const items = getInteractiveItems();
      if (items.length === 0) return;

      if (e.key === 'ArrowDown') {
        e.preventDefault();
        selectedIndex++;
        if (selectedIndex >= items.length) selectedIndex = 0; // Wrap around
        updateSelection();
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        selectedIndex--;
        if (selectedIndex < 0) selectedIndex = items.length - 1; // Wrap around
        updateSelection();
      } else if (e.key === 'Enter') {
        if (selectedIndex > -1) {
          e.preventDefault();
          items[selectedIndex].click();
        }
      }
    });
  }
});
