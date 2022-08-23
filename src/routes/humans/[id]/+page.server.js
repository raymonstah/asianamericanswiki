export async function load({ params }) {
  const post = await import(`../../../../content/humans/${params.id}/index.md`);
  const metadata = post.metadata;
  const content = post.default.render().html;

  return {
    metadata,
    content,
  };
}
