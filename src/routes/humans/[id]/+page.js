/** @type {import('./$types').PageLoad} */

export async function load({ fetch, params }) {
  let human = {};
  await fetch(`https://api-5cwffcuiba-uc.a.run.app/humans/${params.id}`)
    .then((response) => response.json())
    .then((data) => {
      human = data.data;
    })
    .catch((error) => {
      console.log(error);
    });

  return { human: human };
}
