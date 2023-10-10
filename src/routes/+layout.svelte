<script>
  import Header from "$lib/components/Header.svelte";
  import Footer from "$lib/components/Footer.svelte";
  import "$lib/styles/style.css";
  import "../app.postcss";
</script>

<Header />

<main>
  <slot />
</main>

<Footer />
