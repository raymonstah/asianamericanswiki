<script>
  import {
    browserLocalPersistence,
    getAuth,
    GoogleAuthProvider,
    setPersistence,
    signInWithPopup,
  } from "firebase/auth";
  import { goto } from "$app/navigation";

  async function loginWithGoogle() {
    const auth = getAuth();
    setPersistence(auth, browserLocalPersistence)
      .then(() => {
        const provider = new GoogleAuthProvider();
        signInWithPopup(auth, provider).then(() => {
          goto(`/`);
        });
      })
      .catch((error) => {
        console.log(error);
      });
  }
</script>

<svelte:head>
  <title>Login | AsianAmericans.wiki</title>
</svelte:head>

<button on:click={loginWithGoogle}>Login with Google</button>
