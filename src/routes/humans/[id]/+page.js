/** @type {import('./$types').PageLoad} */
import { PUBLIC_BASE_URL } from "$env/static/public";

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

  return { human: human };
}
