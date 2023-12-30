<script>
  import Header from "$lib/components/Header.svelte";
  import Footer from "$lib/components/Footer.svelte";
  import Analytics from "$lib/components/Analytics.svelte";
  import "$lib/styles/style.css";
  import "../app.postcss";
</script>

<Header />

<Analytics />
<main>
  <slot />
</main>

<Footer />
