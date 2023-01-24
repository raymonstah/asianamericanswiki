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

  function contribute() {
    Object.entries(errors);
    for (const key in errors) {
      let val = errors[key];
      if (val) {
        console.log(`${key} is invalid`);
        return;
      }
    }
    console.log("Form received");
  }
</script>

<svelte:head>
  <title>Contribute | AsianAmericans.wiki</title>
</svelte:head>

<article>
  <h1>Contribute an influential Asian American</h1>
  <form on:submit|preventDefault={contribute}>
    <label for="name">Name</label>
    <input id="name" type="text" bind:value={human.name} />

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
    <button class="submit" type="submit">Submit</button>
  </form>
  <p>
    {JSON.stringify(human)}
  </p>
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

  .submit {
    margin-top: 10px;
    font-weight: bold;
    letter-spacing: 0.1rem;
    text-transform: uppercase;
    height: 30px;
  }
  .submit:hover {
    cursor: pointer;
    background-color: #ffe700;
  }
</style>
