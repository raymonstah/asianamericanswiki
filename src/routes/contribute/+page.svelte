<script>
  import BirthdayInput from "../../lib/components/BirthdayInput.svelte";
  import { user, auth } from "$lib/firebase";
  import { PUBLIC_BASE_URL } from "$env/static/public";
  import ethnicities from "$lib/flags.json";
  import tags from "$lib/tags.json";
  import { goto } from "$app/navigation";
  import Tags from "svelte-tags-input";
  import { onMount } from "svelte";
  import AuthCheck from "$lib/components/AuthCheck.svelte";

  let human = {};
  let errors = {};
  let image = null;
  let imageInput;
  const ethnicityList = Object.values(ethnicities)
    .map((countryData) => countryData.ethnicity)
    .filter((ethnicity) => ethnicity !== undefined);

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
            console.log("response", response);
            console.log("image", image);
            if (image && response.data.data.signedUrl) {
              console.log(
                "uploading image to",
                image,
                response.data.data.signedUrl
              );
              const headers = new Headers({
                "Content-Type": image.type,
              });
              fetch(response.data.data.signedUrl, {
                method: "PUT",
                headers: headers,
                body: image,
              })
                .then((response) => console.log(response))
                .catch((error) => {
                  console.log(error);
                });
            }
            human = {}; // only clear on success
            image = null;
            imageInput.value = "";
          })
          .catch((error) => {
            response.hasError = true;
            response.error = error.error;
            console.log(error);
          });
      }
    });
  }

  function handleFileInputChange(event) {
    const fileInput = event.target;
    image = fileInput.files[0];
  }
</script>

<svelte:head>
  <title>Contribute | AsianAmericans.wiki</title>
</svelte:head>

<AuthCheck>
  <article>
    <h1 class="text-2xl">Contribute an influential Asian American</h1>
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
          shortly. View <a href="/humans/{response.data.data.path}"
            >{response.data.data.path}</a
          >.</span
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

      <label for="gender">Gender</label>
      <!-- Create an select for Gender, one of "male", "female", or "nonbinary" -->

      <select
        class="p-1 bg-gray-100 dark:bg-slate-950 dark:text-slate-300"
        required
        id="gender"
        bind:value={human.gender}
      >
        <option value="" disabled>Select a gender</option>
        <option value="male">Male</option>
        <option value="female">Female</option>
        <option value="nonbinary">Nonbinary</option>
      </select>

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

      <label for="imdb">IMDB</label>
      <input
        id="imdb"
        class="bg-gray-100 dark:bg-slate-950 dark:text-slate-300"
        type="url"
        bind:value={human.imdb}
      />

      <label for="image">Image</label>
      <input
        id="image"
        type="file"
        accept=".jpg, .jpeg, .png"
        class="bg-gray-100 dark:bg-slate-950 dark:text-slate-300"
        bind:value={human.image_path}
        bind:this={imageInput}
        on:change={handleFileInputChange}
      />

      <label for="tags">Tags</label>
      <Tags
        id="tags"
        name="tags"
        bind:tags={human.tags}
        onlyUnique="true"
        maxTags={7}
        autoComplete={tags}
        placeholder={"musician comedian engineer actress"}
      />
      <button
        class="bg-transparent hover:bg-yellow-500 text-yellow-600 font-semibold hover:text-white my-4 py-2 px-4 border border-yellow-500 hover:border-transparent"
        type="submit">Submit</button
      >
    </form>
  </article>
</AuthCheck>

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
