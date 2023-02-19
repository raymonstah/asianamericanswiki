<script>
  import BirthdayInput from "../../lib/components/BirthdayInput.svelte";

  let human = {};
  let errors = {};
  const ethnicityList = [
    "Chinese",
    "Vietnamese",
    "Korean",
    "Burmese",
    "Indian",
    "Cambodian",
    "Japanese",
    "Taiwanese",
    "Thai",
    "Filipino",
    "Burmese",
    "Mongolian",
    "Malaysian",
    "Laotian",
    "Indonesian",
  ];
  const tagsList = [
    "author",
    "rapper",
    "musician",
    "singer",
    "actor",
    "comedian",
    "lgbtq",
    "entrepreneur",
    "restaurateur",
    "activist",
    "athlete",
    "ceo",
    "cofounder",
    "chef",
    "designer",
    "director",
    "engineer",
    "film",
    "fitness",
    "founder",
    "model",
    "news",
    "producer",
  ];
  import Tags from "svelte-tags-input";

  import { loggedIn } from "../../store.js";
  let response = {};
  async function contribute() {
    Object.entries(errors);
    for (const key in errors) {
      let val = errors[key];
      if (val) {
        console.log(`${key} is invalid`);
        return;
      }
    }
    // clear out the previous response.
    response = {};
    let token = await getAuth().currentUser.getIdToken();
    const headers = new Headers({
      Authorization: `Bearer ${token}`,
    });
    fetch(`${PUBLIC_BASE_URL}/humans/`, {
      method: "POST",
      headers: headers,
      body: JSON.stringify(human),
    })
      .then((response) => response.json())
      .then((data) => {
        if (data.error) {
          response.hasError = true;
          response.error = data.error;
          return;
        }
        response.success = true;
        response.data = data;
        console.log(data);
      })
      .catch((error) => {
        response.hasError = true;
        response.error = error.error;
        console.log(error);
      });
    // clear form
    human = {};
    console.log(response);
  }

  import { PUBLIC_BASE_URL } from "$env/static/public";
  import { getAuth } from "firebase/auth";
  let userLoggedIn = false;
  loggedIn.subscribe((v) => (userLoggedIn = v));
</script>

<svelte:head>
  <title>Contribute | AsianAmericans.wiki</title>
</svelte:head>

<article>
  <h1 class="text-2xl">Contribute an influential Asian American</h1>
  {#if userLoggedIn}
    {#if response.hasError === true}
      <div
        class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded relative"
        role="alert"
      >
        <strong class="font-bold">Uh oh!</strong>
        <span class="block sm:inline">{response.error}</span>
      </div>
    {:else if response.success === true}
      <div
        class="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded relative"
        role="alert"
      >
        <strong class="font-bold">Success!</strong>
        <span class="block sm:inline"
          >Thanks for your contribution. A moderator will review your submission
          shortly.</span
        >
      </div>
    {/if}
    <form on:submit|preventDefault={contribute}>
      <label for="name">Name</label>
      <input required id="name" type="text" bind:value={human.name} />

      <label for="dob">Date of Birth</label>
      <BirthdayInput bind:invalid={errors.dob} bind:dob={human.dob} />

      <label for="dod">Date of Death</label>
      <input id="dod" type="date" bind:value={human.dod} />

      <label for="ethnicity">Ethnicity</label>
      <Tags
        id="ethnicity"
        name="ethnicity"
        bind:tags={human.ethnicity}
        onlyUnique="true"
        maxTags={7}
        autoComplete={ethnicityList}
      />
      <label for="description">Description</label>
      <textarea
        id="description"
        bind:value={human.description}
        rows="5"
        cols="33"
      />

      <label for="location">Location</label>
      <input id="location" type="text" bind:value={human.location} />

      <label for="website">Website</label>
      <input id="website" type="url" bind:value={human.website} />

      <label for="twitter">Twitter</label>
      <input id="twitter" type="url" bind:value={human.twitter} />

      <label for="tags">Tags</label>
      <Tags
        id="tags"
        name="tags"
        bind:tags={human.tags}
        onlyUnique="true"
        maxTags={7}
        autoComplete={tagsList}
        placeholder={"musician comedian engineer actress"}
      />
      <button
        class="bg-transparent hover:bg-yellow-500 text-yellow-600 font-semibold hover:text-white my-4 py-2 px-4 border border-yellow-500 hover:border-transparent"
        type="submit">Submit</button
      >
    </form>
    <p>
      {JSON.stringify(human)}
    </p>
  {:else}
    <h1>Please log in first.</h1>
  {/if}
</article>

<style>
  form {
    display: flex;
    flex-direction: column;
  }

  label {
    font-weight: bold;
    font-size: 12px;
    margin-top: 20px;
    letter-spacing: 0.1rem;
    text-transform: uppercase;
    margin-bottom: 2px;
  }

  textarea {
    resize: none;
  }
</style>
