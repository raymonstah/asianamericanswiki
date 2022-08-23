<script>
  import Header from "$lib/components/Header.svelte";
  import Footer from "$lib/components/Footer.svelte";
  import "$lib/styles/style.css";
  import Auth from "$lib/components/Auth.svelte";
</script>

<Auth />
<Header />

<main>
  <slot />
</main>

<Footer />

<style>
</style>
