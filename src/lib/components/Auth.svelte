<script>
  import { onMount } from "svelte";
  import { getApps, initializeApp } from "firebase/app";
  import {
    browserLocalPersistence,
    connectAuthEmulator,
    getAuth,
    setPersistence,
  } from "firebase/auth";
  import { loggedIn } from "../../store.js";
  import { PUBLIC_USE_AUTH_EMULATOR } from "$env/static/public";
  onMount(() => {
    if (!getApps().length) {
      initializeApp({
        apiKey: "AIzaSyAzAtLQv_j6TFdkKZyuxG4Yibz9V6VtzRA",
        authDomain: "asianamericans-wiki.firebaseapp.com",
        projectId: "asianamericans-wiki",
        storageBucket: "asianamericans-wiki.appspot.com",
        messagingSenderId: "424340922093",
        appId: "1:424340922093:web:c7a5b00652170e2c9cb6e4",
        measurementId: "G-DNWC1SD6ZZ",
      });
    }
    const auth = getAuth();
    if (PUBLIC_USE_AUTH_EMULATOR === "true") {
      connectAuthEmulator(auth, "http://localhost:8081");
    }
    setPersistence(auth, browserLocalPersistence);
    auth.onAuthStateChanged((user) => {
      if (!user) {
        return;
      }
      loggedIn.set(true);
    });
  });
</script>
