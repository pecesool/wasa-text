import SettingsView from "./views/SettingsView.vue";
import GroupSettingsView from "./views/GroupSettingsView.vue";
import { createRouter, createWebHistory } from "vue-router";
import LoginView from "./views/LoginView.vue";
import ConversationsView from "./views/ConversationsView.vue";
import ChatView from "./views/ChatView.vue";

const routes = [
  { path: "/", redirect: "/login" },
  { path: "/login", component: LoginView },
  { path: "/conversations", component: ConversationsView },
  { path: "/chat/:id", component: ChatView, props: true },
  { path: "/settings", component: SettingsView },
  {    path: "/groups/:id/settings",    component: () => import("./views/GroupSettingsView.vue"),    props: true,  }
  
];

export default createRouter({
  history: createWebHistory(),
  routes,
});