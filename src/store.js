import { writable } from "svelte/store";
import { browser } from "$app/environment";

let loggedInValue;
export const loggedIn = writable(loggedInValue || false);
if (browser) {
  loggedInValue = JSON.parse(localStorage.getItem("loggedIn"));
  loggedIn.subscribe((value) => {
    localStorage.setItem("loggedIn", JSON.stringify(value));
  });
}
