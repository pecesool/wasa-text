<template>
  <div class="container">
    <header class="top">
      <h1>WASAText</h1>
      <div v-if="logged" class="right">
        <button @click="logout">Logout</button>
      </div>
    </header>

    <router-view />
  </div>
</template>

<script>
import { clearToken, isLogged } from "./api";

export default {
  data() {
    return { logged: isLogged() };
  },
  watch: {
    $route() {
      this.logged = isLogged();
    },
  },
  methods: {
    logout() {
      clearToken();
      this.$router.push("/login");
    },
  },
};
</script>

<style>
.container { max-width: 900px; margin: 0 auto; padding: 16px; font-family: Arial, sans-serif; }
.top { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
button { padding: 6px 10px; cursor: pointer; }
input { padding: 6px 10px; }
</style>
