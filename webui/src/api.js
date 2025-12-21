import axios from "axios";

function getToken() {
  return localStorage.getItem("wasa_identifier") || "";
}

export function setToken(t) {
  localStorage.setItem("wasa_identifier", t);
}

export function clearToken() {
  localStorage.removeItem("wasa_identifier");
}

export function isLogged() {
  return !!getToken();
}

const api = axios.create({
  baseURL: __API_URL__,
  timeout: 10000,
});

api.interceptors.request.use((config) => {
  const t = getToken();
  if (t) config.headers.Authorization = `Bearer ${t}`;
  return config;
});

export default api;
