<script lang="ts">
  import { page } from "$app/stores";
  const measurementId = "G-DNWC1SD6ZZ";

  $: {
    if (typeof gtag !== "undefined") {
      gtag("config", measurementId, {
        page_title: document.title,
        page_path: $page.url.pathname,
      });
    }
  }
</script>

<svelte:head>
  <script
    async
    src="https://www.googletagmanager.com/gtag/js?id={measurementId}"
  >
  </script>
  <script>
    window.dataLayer = window.dataLayer || [];

    function gtag() {
      dataLayer.push(arguments);
    }

    gtag("js", new Date());
    gtag("config", "{measurementId}");
  </script>
</svelte:head>
