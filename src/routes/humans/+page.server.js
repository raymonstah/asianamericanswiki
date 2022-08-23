export const load = async ({ url }) => {
  const postRes = await fetch(`${url.origin}/api/humans`);
  const humans = await postRes.json();

  return { humans };
};
