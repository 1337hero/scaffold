import { getDeskData, getInboxData, getNotebooks, getNotebook } from "@/data/mock.js";

export const deskQuery = {
  queryKey: ["desk"],
  queryFn: async () => getDeskData(),
};

export const inboxQuery = {
  queryKey: ["inbox"],
  queryFn: async () => getInboxData(),
};

export const notebooksQuery = {
  queryKey: ["notebooks"],
  queryFn: async () => getNotebooks(),
};

export const notebookQuery = (id) => ({
  queryKey: ["notebook", id],
  queryFn: async () => getNotebook(id),
});
