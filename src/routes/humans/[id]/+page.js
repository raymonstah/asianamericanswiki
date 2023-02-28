/** @type {import('./$types').PageLoad} */
import { PUBLIC_BASE_URL } from "$env/static/public";
import { error } from "@sveltejs/kit";

export async function load({ fetch, params }) {
  let human = {};
  await fetch(`${PUBLIC_BASE_URL}/humans/${params.id}`)
    .then((response) => response.json())
    .then((data) => {
      human = data.data;
    })
    .catch((error) => {
      console.log(error);
    });

  if (!human) {
    throw error(404, {
      message: "Human not found.. Are you sure you have the right path?",
    });
  }
  return { human: human };
}
