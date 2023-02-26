export default function truncate(str, n) {
  if (!str) {
    return "";
  }
  return str.length > n ? str.slice(0, n - 1) + "..." : str;
}
