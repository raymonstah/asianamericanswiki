import { writable } from "svelte/store";
import { browser } from "$app/env";

let loggedInValue;
export const loggedIn = writable(loggedInValue || false);
if (browser) {
  loggedInValue = JSON.parse(localStorage.getItem("loggedIn"));
  console.log("loggedInValue", loggedInValue);
  loggedIn.subscribe((value) => {
    localStorage.setItem("loggedIn", JSON.stringify(value));
  });
}
