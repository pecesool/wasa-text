
<template>
  <div>
    <h2>My Settings</h2>

    <p v-if="err" style="color:red">{{ err }}</p>
    <p v-if="ok" style="color:green">{{ ok }}</p>

    <div class="card">
      <h3>Profile photo</h3>

      <div class="avatar">
        <img v-if="photo" :src="photo" />
        <div v-else class="fallback"></div>
      </div>

      <div class="row">
        <input type="file" accept="image/*" @change="onPickPhoto" />
        <button type="button" @click="savePhoto" :disabled="!photoPicked">Save photo</button>
        <button type="button" @click="removePhoto">Remove photo</button>
      </div>
      <div class="small">If you don’t set it, it will appear as a black rectangle.</div>
    </div>

    <div class="card">
      <h3>Change username</h3>
      <div class="row">
        <input v-model="newName" placeholder="New username..." />
        <button type="button" @click="saveName">Save name</button>
      </div>
      <div class="small">Name must be unique (backend returns 409 if already used).</div>
    </div>

    <div class="row">
      <button type="button" @click="$router.push('/conversations')">← Back</button>
    </div>
  </div>
</template>

<script>
import api, { isLogged } from "../api";

export default {
  data() {
    return {
      newName: "",
      photo: "",
      photoPicked: false,
      err: "",
      ok: "",
    };
  },
  async mounted() {
    if (!isLogged()) {
      this.$router.push("/login");
      return;
    }
    // We don't have GET /me in backend, so we just keep current username from localStorage
    this.newName = localStorage.getItem("wasa_username") || "";
  },
  methods: {
    clearMsgs() {
      this.err = "";
      this.ok = "";
    },

    onPickPhoto(e) {
      this.clearMsgs();
      const file = e.target.files && e.target.files[0];
      if (!file) return;

      const reader = new FileReader();
      reader.onload = () => {
        this.photo = String(reader.result || "");
        this.photoPicked = true;
      };
      reader.onerror = () => (this.err = "Failed to read file");
      reader.readAsDataURL(file);
    },

    async savePhoto() {
      this.clearMsgs();
      try {
        await api.put("/me/photo", { photo: this.photo || "" });
        this.ok = "Photo saved";
        this.photoPicked = false;
      } catch (e) {
        this.err = "Failed to save photo";
      }
    },

    async removePhoto() {
      this.clearMsgs();
      try {
        await api.put("/me/photo", { photo: "" });
        this.photo = "";
        this.photoPicked = false;
        this.ok = "Photo removed";
      } catch (e) {
        this.err = "Failed to remove photo";
      }
    },

    async saveName() {
      this.clearMsgs();
      const n = (this.newName || "").trim();
      if (n.length < 3) {
        this.err = "Name too short";
        return;
      }

      try {
        await api.put("/me/name", { name: n });
        localStorage.setItem("wasa_username", n);
        this.ok = "Name updated";
      } catch (e) {
        // if backend returns 409
        this.err = "Failed to update name (maybe already used?)";
      }
    },
  },
};
</script>

<style>
.card { border: 1px solid #ddd; border-radius: 8px; padding: 12px; margin: 12px 0; }
.row { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; margin-top: 10px; }
input { padding: 6px 10px; }
.small { font-size: 12px; opacity: 0.7; margin-top: 6px; }

.avatar {
  width: 80px;
  height: 80px;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid #eee;
  margin-top: 8px;
}
.avatar img { width: 100%; height: 100%; object-fit: cover; display: block; }
.fallback { width: 100%; height: 100%; background: #000; }
</style>
