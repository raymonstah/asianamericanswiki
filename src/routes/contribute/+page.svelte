<script>
  import BirthdayInput from "../../lib/components/BirthdayInput.svelte";
  import { user } from "$lib/firebase";
  import { PUBLIC_BASE_URL } from "$env/static/public";

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
    if (human.location) {
      human.location = human.location.split(",");
    }
    user.subscribe(async (user) => {
      if (user) {
        const headers = new Headers({
          Authorization: `Bearer ${user.accessToken}`,
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
    });
  }
</script>

<svelte:head>
  <title>Contribute | AsianAmericans.wiki</title>
</svelte:head>

<article>
  <h1 class="text-2xl">Contribute an influential Asian American</h1>
  {#if $user}
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
      <input
        class="bg-gray-100 dark:bg-slate-950 dark:text-slate-300"
        required
        id="name"
        type="text"
        bind:value={human.name}
      />

      <label for="dob">Date of Birth</label>
      <BirthdayInput
        class="bg-gray-100 dark:bg-slate-950 dark:text-slate-300"
        bind:invalid={errors.dob}
        bind:dob={human.dob}
      />

      <label for="dod">Date of Death</label>
      <input
        class="bg-gray-100 dark:bg-slate-950 dark:text-slate-300"
        id="dod"
        type="date"
        bind:value={human.dod}
      />

      <label for="ethnicity">Ethnicity</label>
      <Tags
        class="bg-gray-100 dark:bg-slate-950 dark:text-slate-300"
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
        class="bg-gray-100 dark:bg-slate-950 dark:text-slate-300"
        bind:value={human.description}
        rows="5"
        cols="33"
      />

      <label for="location">Location</label>
      <input
        id="location"
        class="bg-gray-100 dark:bg-slate-950 dark:text-slate-300"
        type="text"
        bind:value={human.location}
      />

      <label for="website">Website</label>
      <input
        id="website"
        class="bg-gray-100 dark:bg-slate-950 dark:text-slate-300"
        type="url"
        bind:value={human.website}
      />

      <label for="twitter">Twitter</label>
      <input
        id="twitter"
        class="bg-gray-100 dark:bg-slate-950 dark:text-slate-300"
        type="url"
        bind:value={human.twitter}
      />

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
    <!-- Uncomment below to preview form JSON. -->
    <!-- <p>
      {JSON.stringify(human)}
    </p> -->
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
