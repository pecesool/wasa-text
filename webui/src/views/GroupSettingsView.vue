
<template>
  <div>
    <button type="button" @click="$router.push(`/chat/${id}`)">← Back to chat</button>

    <h2 style="margin-top:10px">Group Settings</h2>
    <p style="opacity:0.7">Group/Conversation ID: {{ id }}</p>

    <p v-if="err" style="color:red">{{ err }}</p>
    <p v-if="ok" style="color:green">{{ ok }}</p>

    <div v-if="loading">Loading...</div>

    <div v-else>
      <div class="card">
        <h3>Group photo</h3>
        <div class="avatar">
          <img v-if="photo" :src="photo" />
          <div v-else class="fallback"></div>
        </div>

        <div class="row">
          <input type="file" accept="image/*" @change="onPickPhoto" />
          <button type="button" @click="savePhoto" :disabled="!photoPicked">Save photo</button>
          <button type="button" @click="removePhoto">Remove photo</button>
        </div>
      </div>

      <div class="card">
        <h3>Rename group</h3>
        <div class="row">
          <input v-model="newName" placeholder="New group name..." />
          <button type="button" @click="saveName">Save name</button>
        </div>
      </div>

      <div class="card">
        <h3>Add member</h3>
        <div class="row">
          <input v-model="addUser" placeholder="Username to add..." />
          <button type="button" @click="addMember">Add</button>
        </div>

        <div class="small">Members:</div>
        <ul>
          <li v-for="m in members" :key="m">{{ m }}</li>
        </ul>
      </div>

      <div class="card">
        <h3>Leave group</h3>
        <button type="button" class="danger" @click="leave">Leave</button>
      </div>
    </div>
  </div>
</template>

<script>
import api, { isLogged } from "../api";

export default {
  props: ["id"],
  data() {
    return {
      loading: true,
      err: "",
      ok: "",

      // current data
      title: "",
      photo: "",
      members: [],

      // edits
      newName: "",
      photoPicked: false,
      addUser: "",
    };
  },
  async mounted() {
    if (!isLogged()) {
      this.$router.push("/login");
      return;
    }
    await this.load();
  },
  methods: {
    clearMsgs() {
      this.err = "";
      this.ok = "";
    },

    async load() {
      this.clearMsgs();
      this.loading = true;
      try {
        const res = await api.get(`/conversations/${this.id}`);
        if (!res.data.isGroup) {
          this.err = "This is not a group chat.";
          this.loading = false;
          return;
        }
        this.title = res.data.title || "";
        this.newName = this.title;
        this.photo = res.data.photo || "";
        this.members = res.data.members || [];
      } catch (e) {
        this.err = "Failed to load group";
      } finally {
        this.loading = false;
      }
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
        await api.put(`/groups/${this.id}/photo`, { photo: this.photo || "" });
        this.ok = "Group photo saved";
        this.photoPicked = false;
        await this.load();
      } catch (e) {
        this.err = "Failed to save group photo";
      }
    },

    async removePhoto() {
      this.clearMsgs();
      try {
        await api.put(`/groups/${this.id}/photo`, { photo: "" });
        this.photo = "";
        this.photoPicked = false;
        this.ok = "Group photo removed";
        await this.load();
      } catch (e) {
        this.err = "Failed to remove group photo";
      }
    },

    async saveName() {
      this.clearMsgs();
      const n = (this.newName || "").trim();
      if (!n) {
        this.err = "Name required";
        return;
      }
      try {
        await api.put(`/groups/${this.id}/name`, { name: n });
        this.ok = "Group name saved";
        await this.load();
      } catch (e) {
        this.err = "Failed to save group name";
      }
    },

    async addMember() {
      this.clearMsgs();
      const u = (this.addUser || "").trim();
      if (!u) return;

      try {
        await api.post(`/groups/${this.id}/members`, { username: u });
        this.addUser = "";
        this.ok = "User added";
        await this.load();
      } catch (e) {
        this.err = "Failed to add user (maybe not found?)";
      }
    },

    async leave() {
      this.clearMsgs();
      if (!confirm("Leave this group?")) return;

      try {
        await api.post(`/groups/${this.id}/leave`, {});
        this.ok = "You left the group";
        this.$router.push("/conversations");
      } catch (e) {
        this.err = "Failed to leave group";
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

.danger { border: 1px solid #d66; padding: 6px 10px; }
</style>
