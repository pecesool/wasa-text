<template>
  <div>
    <h2>Login (fake)</h2>
    <p>Type a username (3–16). This creates the user or reuses it.</p>

    <form @submit.prevent="login">
      <input v-model="name" placeholder="Mario" />
      <button type="submit">Login</button>
    </form>

    <p v-if="err" style="color:red">{{ err }}</p>
  </div>
</template>

<script>
import api, { setToken } from "../api";

export default {
  data() {
    return { name: "", err: "" };
  },
  methods: {
    async login() {
      this.err = "";
      const n = this.name.trim();
      if (n.length < 3 || n.length > 16) {
        this.err = "Name must be 3..16 chars";
        return;
      }
      try {
        const res = await api.post("/session", { name: n });
        setToken(res.data.identifier);
        this.$router.push("/conversations");
      } catch (e) {
        this.err = "Login failed";
      }
    },
  },
};
</script>
