/** @type {import('./$types').PageLoad} */
import { PUBLIC_BASE_URL } from "$env/static/public";
import { error } from "@sveltejs/kit";

export async function load({ fetch, params }) {
  let mostViewedHumans = {};
  let recentlyAdded = {};
  await fetch(`${PUBLIC_BASE_URL}/humans/?orderBy=views&direction=desc&limit=8`)
    .then((response) => response.json())
    .then((data) => {
      mostViewedHumans = data.data;
    })
    .catch((error) => {
      console.log(error);
    });

  if (!mostViewedHumans) {
    throw error(500, {
      message: "oops.. something went wrong.",
    });
  }

  await fetch(
    `${PUBLIC_BASE_URL}/humans/?orderBy=created_at&direction=desc&limit=8`
  )
    .then((response) => response.json())
    .then((data) => {
      recentlyAdded = data.data;
    })
    .catch((error) => {
      console.log(error);
    });
  if (!recentlyAdded) {
    throw error(500, {
      message: "oops.. something went wrong.",
    });
  }
  return { mostViewedHumans: mostViewedHumans, recentlyAdded: recentlyAdded };
}
