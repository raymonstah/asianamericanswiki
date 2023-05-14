<script>
  import { PUBLIC_BASE_URL } from "$env/static/public";
  import { onMount } from "svelte";
  import { getAuth, onAuthStateChanged } from "firebase/auth";
  let drafts = [];

  async function loadDrafts() {
    const auth = getAuth();
    onAuthStateChanged(auth, function (user) {
      if (user) {
        user.getIdToken().then(function (data) {
          const headers = new Headers({
            Authorization: `Bearer ${data}`,
          });
          fetch(`${PUBLIC_BASE_URL}/humans/drafts`, {
            headers: headers,
          })
            .then((response) => {
              return response.json();
            })
            .then((data) => {
              drafts = data.data;
            });
        });
      }
    });
  }

  function review(humanId, rev) {
    const auth = getAuth();
    onAuthStateChanged(auth, function (user) {
      if (user) {
        user.getIdToken().then(function (data) {
          const headers = new Headers({
            Authorization: `Bearer ${data}`,
          });
          fetch(`${PUBLIC_BASE_URL}/humans/${humanId}/review`, {
            headers: headers,
            method: "POST",
            body: JSON.stringify({ review: rev }),
          }).catch((err) => {
            console.log(err);
          });
        });
      }
    });

    // remove the draft from the ui
    drafts = drafts.filter((draft) => draft.id !== humanId);
  }

  onMount(async () => {
    loadDrafts();
  });
</script>

<svelte:head>
  <title>Admin | AsianAmericans.wiki</title>
</svelte:head>
<article class="max-w-lg mx-auto">
  <div class="text-left leading-relaxed">
    <h1 class="text-4xl font-extrabold mb-4">Draft Requests</h1>
    {#each drafts as draft}
      <div
        class="max-w-sm p-4 bg-white border border-gray-200 rounded-lg shadow hover:bg-gray-100 
        flex flex-row space-x-4 items-center
        dark:bg-gray-800 dark:border-gray-700 dark:hover:bg-gray-700"
      >
        <h2 class="text-xl text-gray-900 dark:text-white">
          <a class="name" href="humans/{draft.path}">{draft.name}</a>
        </h2>
        <button
          on:click={review(draft.id, "approve")}
          class="bg-green-500 hover:bg-green-700 text-white font-bold py-2 px-4 rounded"
          >Approve</button
        >
        <button
          on:click={review(draft.id, "reject")}
          class="bg-red-500 hover:bg-red-700 text-white font-bold py-2 px-4 rounded"
          >Decline</button
        >
      </div>
    {/each}
  </div>
</article>
