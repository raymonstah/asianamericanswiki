export default function debounce(node, params) {
  let timer;

  return {
    update() {
      clearTimeout(timer);
      timer = setTimeout(params.func, params.duration);
    },
    destroy() {
      clearTimeout(timer);
    },
  };
}
