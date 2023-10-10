// from fireship.io
import { initializeApp } from "@firebase/app";
import {
  User,
  connectAuthEmulator,
  getAuth,
  onAuthStateChanged,
} from "@firebase/auth";
import { writable } from "svelte/store";
import { PUBLIC_USE_AUTH_EMULATOR } from "$env/static/public";
const firebaseConfig = {
  apiKey: "AIzaSyAzAtLQv_j6TFdkKZyuxG4Yibz9V6VtzRA",
  authDomain: "asianamericans-wiki.firebaseapp.com",
  projectId: "asianamericans-wiki",
  storageBucket: "asianamericans-wiki.appspot.com",
  messagingSenderId: "424340922093",
  appId: "1:424340922093:web:c7a5b00652170e2c9cb6e4",
  measurementId: "G-DNWC1SD6ZZ",
};
export const app = initializeApp(firebaseConfig);
export const auth = getAuth();
if (PUBLIC_USE_AUTH_EMULATOR === "true") {
  connectAuthEmulator(auth, "http://localhost:8081");
}

function userStore() {
  let unsubscribe: () => void;

  if (!auth || !globalThis.window) {
    console.warn("Auth is not initialized or not in browser");
    const { subscribe } = writable<User | null>(null);
    return {
      subscribe,
    };
  }
  const { subscribe } = writable(auth?.currentUser, (set) => {
    unsubscribe = onAuthStateChanged(auth, (user) => {
      set(user);
    });

    return () => unsubscribe();
  });
  return {
    subscribe,
  };
}

export const user = userStore();
