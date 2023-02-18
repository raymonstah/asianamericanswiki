export const prerender = false;
export function load({ url }) {
  const query = url.searchParams.get("query");
  return {
    query,
  };
}
