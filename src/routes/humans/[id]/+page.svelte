<script>
  import { PUBLIC_BASE_URL } from "$env/static/public";
  import SvelteMarkdown from "svelte-markdown";
  import Chip from "../../../lib/components/Chip.svelte";
  import Affiliate from "../../../lib/components/Affiliate.svelte";
  import dayjs from "dayjs";
  import relativeTime from "dayjs/plugin/relativeTime";
  import { user } from "$lib/firebase";
  import { onMount } from "svelte";
  dayjs.extend(relativeTime);

  export let data;

  const humanFields = [
    { key: "aliases", label: "Aliases", isArray: true },
    { key: "gender", label: "Gender" },
    { key: "dob", label: "Born", isDate: true },
    { key: "dod", label: "Died", isDate: true },
    { key: "ethnicity", label: "Ethnicity", isArray: true },
    { key: "birthLocation", label: "Birth Location" },
    { key: "location", label: "Location", isArray: true },
    { key: "createdAt", label: "Created", isRelativeDate: true },
    { key: "updatedAt", label: "Last Updated", isRelativeDate: true },
  ];

  function formatDateString(dateString) {
    const parts = dateString.split("-");
    const year = parts[0];
    const month = parts[1];
    const day = parts[2];

    if (year && month && day) {
      return dayjs(dateString).format("MMMM D, YYYY");
    } else if (year && month) {
      return `${dayjs(dateString).format("MMMM")}, ${year}`;
    } else if (year) {
      return year;
    } else {
      return "";
    }
  }

  let isHumanSaved = false;
  function saveHuman() {
    isHumanSaved = !isHumanSaved;

    user.subscribe(async (currentUser) => {
      if (!currentUser) {
        alert("You must be logged in to save a human.");
        return;
      }

      const headers = new Headers({
        Authorization: `Bearer ${currentUser.accessToken}`,
      });

      const method = isHumanSaved ? "POST" : "DELETE";
      const url = `${PUBLIC_BASE_URL}/humans/${data.human.id}/save`;

      try {
        const response = await fetch(url, {
          method: method,
          headers: headers,
        });

        if (!response.ok) {
          console.error("Error:", response.statusText);
        }
      } catch (error) {
        console.error("Error:", error);
      }
    });
  }

  onMount(async () => {
    const headers = new Headers();
    user.subscribe(async (user) => {
      if (user) {
        headers.append("Authorization", `Bearer ${user.accessToken}`);

        // Get the user to see if they have saved this human.
        let u = await fetch(`${PUBLIC_BASE_URL}/user`, {
          headers: headers,
        }).then((response) => {
          return response.json();
        });
        if (u.data.saved.some((h) => h.human_id === data.human.id)) {
          isHumanSaved = true;
        }
      }
    });

    try {
      const response = await fetch(
        `${PUBLIC_BASE_URL}/humans/${data.human.id}/view`,
        {
          method: "POST",
          headers: headers,
        }
      );

      if (!response.ok) {
        throw new Error(`HTTP error! Status: ${response.status}`);
      }
    } catch (error) {
      console.error(error);
    }
  });
</script>

<svelte:head>
  <title>{data.human.name} | AsianAmericans.wiki</title>
  <meta name="description" content={data.human.description} />

  <!-- Facebook Meta Tags -->
  <meta
    property="og:url"
    content="https://asianamericans.wiki/humans/{data.human.path}"
  />
  <meta property="og:type" content="website" />
  <meta property="og:title" content={data.human.name} />
  <meta property="og:description" content={data.human.description} />
  <meta property="og:image" content={data.human.featuredImage} />

  <!-- Twitter Meta Tags -->
  <meta name="twitter:card" content="summary_large_image" />
  <meta
    property="twitter:url"
    content="https://asianamericans.wiki/humans/{data.human.path}"
  />
  <meta name="twitter:title" content={data.human.name} />
  <meta name="twitter:description" content={data.human.description} />
  <meta name="twitter:image" content={data.human.featuredImage} />
</svelte:head>

<article class="max-w-2xl">
  <!-- Header -->
  <h1 class="text-2xl">{data.human.name}</h1>
  <button class="cursor-pointer" on:click={saveHuman}>
    <div class="my-2">
      <svg
        fill="currentColor"
        class={isHumanSaved ? "text-amber-300 " : "text-gray-500 "}
        xmlns="http://www.w3.org/2000/svg"
        height="2em"
        viewBox="0 0 512 512"
        ><!--! Font Awesome Free 6.4.2 by @fontawesome - https://fontawesome.com License - https://fontawesome.com/license (Commercial License) Copyright 2023 Fonticons, Inc. --><path
          d="M47.6 300.4L228.3 469.1c7.5 7 17.4 10.9 27.7 10.9s20.2-3.9 27.7-10.9L464.4 300.4c30.4-28.3 47.6-68 47.6-109.5v-5.8c0-69.9-50.5-129.5-119.4-141C347 36.5 300.6 51.4 268 84L256 96 244 84c-32.6-32.6-79-47.5-124.6-39.9C50.5 55.6 0 115.2 0 185.1v5.8c0 41.5 17.2 81.2 47.6 109.5z"
        /></svg
      >
    </div>
  </button>
  {#if data.human.featuredImage}
    <img class="w-sm" src={data.human.featuredImage} alt={data.human.name} />
  {/if}
  <!-- Table -->
  <table
    class="table-fixed w-full text-sm text-left text-gray-500 dark:text-white bg-gray-200 dark:bg-gray-800"
  >
    {#each humanFields as field}
      {#if data.human[field.key]}
        <tr
          class="border-b dark:bg-gray-800 border-gray-300 dark:border-gray-700"
        >
          <th
            class="dark:text-white px-4 py-4 w-1/3 font-medium text-gray-900 whitespace-nowrap"
            >{field.label}</th
          >
          <td
            class="dark:text-white px-4 py-4 w-2/3 font-medium text-gray-900 whitespace-nowrap"
          >
            {#if field.isArray}
              <ul class="flex flex-row flex-wrap gap-y-3">
                {#each data.human[field.key] as item}
                  <li class="space-y-4">
                    <Chip><a href="/search?query={item}">{item}</a></Chip>
                  </li>
                {/each}
              </ul>
            {:else if field.isRelativeDate}
              {dayjs(data.human[field.key]).fromNow()}
            {:else if field.isDate}
              {#if data.human[field.key]}
                {formatDateString(data.human[field.key])}
                <!-- parse the "dob" to determine how old they are assuming "dod" is not present. -->
                {#if data.human.dob && !data.human.dod}
                  (age {dayjs().diff(data.human.dob, "year")} years)
                {/if}
              {/if}
            {:else}
              {data.human[field.key]}
            {/if}
          </td>
        </tr>
      {/if}
    {/each}

    <!-- Socials Row -->
    <!-- Check if human.socials is present -->
    {#if data.human.socials && Object.keys(data.human.socials).length > 0}
      <tr class="dark:bg-gray-800">
        <th
          class="dark:text-white px-4 py-4 w-1/3 font-medium text-gray-900 whitespace-nowrap"
          >Socials</th
        >
        <td
          class="dark:text-white px-4 py-4 w-2/3 font-medium text-gray-900 whitespace-nowrap"
        >
          <ul class="flex flex-row space-x-2 align-middle items-center">
            {#if data.human.socials.x}
              <a target="_blank" href={data.human.socials.x}
                ><svg
                  xmlns="http://www.w3.org/2000/svg"
                  fill="currentColor"
                  height="1.5em"
                  viewBox="0 0 512 512"
                  ><!--! Font Awesome Free 6.4.2 by @fontawesome - https://fontawesome.com License - https://fontawesome.com/license (Commercial License) Copyright 2023 Fonticons, Inc. --><path
                    d="M389.2 48h70.6L305.6 224.2 487 464H345L233.7 318.6 106.5 464H35.8L200.7 275.5 26.8 48H172.4L272.9 180.9 389.2 48zM364.4 421.8h39.1L151.1 88h-42L364.4 421.8z"
                  /></svg
                ></a
              >
            {/if}
            {#if data.human.socials.website}
              <a target="_blank" href={data.human.socials.website}
                ><svg
                  xmlns="http://www.w3.org/2000/svg"
                  height="1.5em"
                  fill="currentColor"
                  viewBox="0 0 576 512"
                  ><!--! Font Awesome Free 6.4.2 by @fontawesome - https://fontawesome.com License - https://fontawesome.com/license (Commercial License) Copyright 2023 Fonticons, Inc. --><path
                    d="M575.8 255.5c0 18-15 32.1-32 32.1h-32l.7 160.2c0 2.7-.2 5.4-.5 8.1V472c0 22.1-17.9 40-40 40H456c-1.1 0-2.2 0-3.3-.1c-1.4 .1-2.8 .1-4.2 .1H416 392c-22.1 0-40-17.9-40-40V448 384c0-17.7-14.3-32-32-32H256c-17.7 0-32 14.3-32 32v64 24c0 22.1-17.9 40-40 40H160 128.1c-1.5 0-3-.1-4.5-.2c-1.2 .1-2.4 .2-3.6 .2H104c-22.1 0-40-17.9-40-40V360c0-.9 0-1.9 .1-2.8V287.6H32c-18 0-32-14-32-32.1c0-9 3-17 10-24L266.4 8c7-7 15-8 22-8s15 2 21 7L564.8 231.5c8 7 12 15 11 24z"
                  /></svg
                ></a
              >
            {/if}
            {#if data.human.socials.imdb}
              <a target="_blank" href={data.human.socials.imdb}
                ><svg
                  xmlns="http://www.w3.org/2000/svg"
                  height="2em"
                  fill="currentColor"
                  viewBox="0 0 448 512"
                  ><!--! Font Awesome Free 6.4.2 by @fontawesome - https://fontawesome.com License - https://fontawesome.com/license (Commercial License) Copyright 2023 Fonticons, Inc. --><path
                    d="M89.5 323.6H53.93V186.2H89.5V323.6zM156.1 250.5L165.2 186.2H211.5V323.6H180.5V230.9L167.1 323.6H145.8L132.8 232.9L132.7 323.6H101.5V186.2H147.6C148.1 194.5 150.4 204.3 151.9 215.6L156.1 250.5zM223.7 323.6V186.2H250.3C267.3 186.2 277.3 187.1 283.3 188.6C289.4 190.3 294 192.8 297.2 196.5C300.3 199.8 302.3 203.1 303 208.5C303.9 212.9 304.4 221.6 304.4 234.7V282.9C304.4 295.2 303.7 303.4 302.5 307.6C301.4 311.7 299.4 315 296.5 317.3C293.7 319.7 290.1 321.4 285.8 322.3C281.6 323.1 275.2 323.6 266.7 323.6H223.7zM259.2 209.7V299.1C264.3 299.1 267.5 298.1 268.6 296.8C269.7 294.8 270.4 289.2 270.4 280.1V226.8C270.4 220.6 270.3 216.6 269.7 214.8C269.4 213 268.5 211.8 267.1 210.1C265.7 210.1 263 209.7 259.2 209.7V209.7zM316.5 323.6V186.2H350.6V230.1C353.5 227.7 356.7 225.2 360.1 223.5C363.7 222 368.9 221.1 372.9 221.1C377.7 221.1 381.8 221.9 385.2 223.3C388.6 224.8 391.2 226.8 393.2 229.5C394.9 232.1 395.9 234.8 396.3 237.3C396.7 239.9 396.1 245.3 396.1 253.5V292.1C396.1 300.3 396.3 306.4 395.3 310.5C394.2 314.5 391.5 318.1 387.5 320.1C383.4 324 378.6 325.4 372.9 325.4C368.9 325.4 363.7 324.5 360.2 322.9C356.7 321.1 353.5 318.4 350.6 314.9L348.5 323.6L316.5 323.6zM361.6 302.9C362.3 301.1 362.6 296.9 362.6 290.4V255C362.6 249.4 362.3 245.5 361.5 243.8C360.8 241.9 357.8 241.1 355.7 241.1C353.7 241.1 352.3 241.9 351.6 243.4C351 244.9 350.6 248.8 350.6 255V291.4C350.6 297.5 351 301.4 351.8 303C352.4 304.7 353.9 305.5 355.9 305.5C358.1 305.5 360.1 304.7 361.6 302.9L361.6 302.9zM418.4 32.04C434.1 33.27 447.1 47.28 447.1 63.92V448.1C447.1 464.5 435.2 478.5 418.9 479.1C418.6 479.1 418.4 480 418.1 480H29.88C29.6 480 29.32 479.1 29.04 479.9C13.31 478.5 1.093 466.1 0 449.7L.0186 61.78C1.081 45.88 13.82 33.09 30.26 31.1H417.7C417.9 31.1 418.2 32.01 418.4 32.04L418.4 32.04zM30.27 41.26C19 42.01 10.02 51.01 9.257 62.4V449.7C9.63 455.1 11.91 460.2 15.7 464C19.48 467.9 24.51 470.3 29.89 470.7H418.1C429.6 469.7 438.7 459.1 438.7 448.1V63.91C438.7 58.17 436.6 52.65 432.7 48.45C428.8 44.24 423.4 41.67 417.7 41.26L30.27 41.26z"
                  /></svg
                ></a
              >
            {/if}
          </ul>
        </td>
      </tr>
    {/if}
  </table>

  <!-- Description -->
  <div class="text-left px-4 py-4 space-y-4">
    {#if data.human.description}
      <SvelteMarkdown source={data.human.description} />
    {/if}
  </div>

  <!-- Tags -->
  <div class="py-4">
    <ul class="flex flex-row">
      {#if data.human.tags}
        {#each data.human.tags as tag}
          <li>
            <Chip><a href="/search?query={tag}">{tag}</a></Chip>
          </li>
        {/each}
      {/if}
    </ul>
  </div>

  <!-- Affiliate Links -->
  {#if data.human.affiliates.length > 0}
    <h2 class="mt-4 text-xl">Affiliate Links</h2>
    <div
      class="md:flex md:flex-row md:justify-between dark:text-white bg-gray-200 dark:bg-gray-800"
    >
      {#each data.human.affiliates as affiliate}
        <div class="max-w-md mx-auto my-2 p-6 rounded shadow-lg">
          <Affiliate
            image={affiliate.image}
            url={affiliate.url}
            name={affiliate.name}
          />
        </div>
      {/each}
    </div>
  {/if}
</article>
