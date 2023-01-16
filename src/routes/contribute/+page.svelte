<script>
  let human = {};
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
</script>

<svelte:head>
  <title>Contribute | AsianAmericans.wiki</title>
</svelte:head>

<h1>Contribute</h1>
<p>Contribute an influential Asian American</p>
<form>
  <label for="name">Name</label>
  <input id="name" type="text" bind:value={human.name} />

  <label for="dob">Date of Birth</label>
  <input id="dob" type="date" bind:value={human.dob} />

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
  <input id="website" type="text" bind:value={human.website} />

  <label for="twitter">Twitter</label>
  <input id="twitter" type="text" bind:value={human.twitter} />

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
  <button>Submit</button>
</form>
<p>
  {JSON.stringify(human)}
</p>

<style>
  form {
    display: flex;
    flex-direction: column;
  }
  label {
    font-weight: bold;
  }

  input,
  textarea {
    margin-bottom: 10px;
  }
</style>
