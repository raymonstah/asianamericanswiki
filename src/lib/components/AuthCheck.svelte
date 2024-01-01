<script lang="ts">
  import { user } from "$lib/firebase";
</script>

{#if $user}
  <slot />
{:else}
  <div
    class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded relative"
    role="alert"
  >
    <strong class="font-bold">Uh oh!</strong>
    <span class="block sm:inline">You must be signed in to view this page.</span
    >
  </div>
  <a
    class="my-2 bg-blue-500 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded"
    href="/login">Sign in</a
  >
{/if}
