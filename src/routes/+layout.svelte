<script>
  import Header from "$lib/components/Header.svelte";
  import Footer from "$lib/components/Footer.svelte";
  import Analytics from "$lib/components/Analytics.svelte";
  import "$lib/styles/style.css";
  import "../app.postcss";
  import { dev } from "$app/environment";

  // include and initialize the rollbar library with your access token
  import Rollbar from "rollbar";
  var mode = "prod";
  if (dev) {
    mode = "dev";
  }
  var rollbar = new Rollbar({
    accessToken: "e1082079233c44628d29032fc1847ca7",
    captureUncaught: true,
    captureUnhandledRejections: true,
    payload: {
      environment: mode,
    },
  });
</script>

<Header />

<Analytics />
<main>
  <slot />
</main>

<Footer />
