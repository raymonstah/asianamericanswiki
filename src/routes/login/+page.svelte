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
<button
  class="px-4 py-2 border flex gap-2 rounded-lg text-slate-700 dark:text-slate-200 border-slate-400 hover:text-slate-900 hover:shadow transition duration-150"
  on:click={loginWithGoogle}
>
  <img
    class="w-6 h-6"
    src="https://www.svgrepo.com/show/475656/google-color.svg"
    loading="lazy"
    alt="google logo"
  />
  <span>Login with Google</span>
</button>
