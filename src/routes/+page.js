/** @type {import('./$types').PageLoad} */
import { PUBLIC_BASE_URL } from "$env/static/public";
import { error } from "@sveltejs/kit";

export async function load({ fetch, params }) {
  let mostViewedHumans = {};
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
  return { mostViewedHumans: mostViewedHumans };
}
