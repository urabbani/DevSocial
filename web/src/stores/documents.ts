import { create } from 'zustand';
import { api, type CodeDocument } from '../api/client';

interface DocumentViewer {
  userId: number;
  username: string;
  connectedAt: number;
}

interface DocumentState {
  documents: CodeDocument[];
  activeDocumentId: number | null;
  viewers: Record<number, DocumentViewer[]>; // document_id -> viewers
  editors: Record<number, string>; // document_id -> username of person editing
  loading: boolean;
  error: string | null;

  fetchDocuments: (workspaceId: number) => Promise<void>;
  createDocument: (workspaceId: number, data: CreateDocumentInput) => Promise<CodeDocument>;
  getDocument: (id: number) => Promise<CodeDocument>;
  updateDocument: (id: number, data: UpdateDocumentInput) => Promise<CodeDocument>;
  deleteDocument: (id: number) => Promise<void>;
  setActiveDocument: (id: number | null) => void;
  handleWSDocEvent: (event: WSMessage) => void;
  clearViewers: (documentId: number) => void;
  clearEditors: (documentId: number) => void;
}

interface CreateDocumentInput {
  title: string;
  filename: string;
  language: string;
  content: string;
}

interface UpdateDocumentInput {
  title?: string;
  content?: string;
  language?: string;
  version: number;
}

interface WSMessage {
  type: string;
  document_id?: number;
  user_id?: number;
  content?: string; // username for doc_open
  cursor?: { line: number; col: number };
}

export const useDocumentStore = create<DocumentState>((set) => ({
  documents: [],
  activeDocumentId: null,
  viewers: {},
  editors: {},
  loading: false,
  error: null,

  fetchDocuments: async (workspaceId: number) => {
    set({ loading: true, error: null });
    try {
      const documents = await api.listDocuments(workspaceId);
      set({ documents, loading: false });
    } catch (err) {
      set({ error: (err as Error).message, loading: false });
    }
  },

  createDocument: async (workspaceId: number, data: CreateDocumentInput) => {
    set({ loading: true, error: null });
    try {
      const doc = await api.createDocument(workspaceId, data);
      set((s) => ({
        documents: [...s.documents, doc],
        activeDocumentId: doc.id,
        loading: false,
      }));
      return doc;
    } catch (err) {
      set({ error: (err as Error).message, loading: false });
      throw err;
    }
  },

  getDocument: async (id: number) => {
    set({ loading: true, error: null });
    try {
      const doc = await api.getDocument(id);
      set((s) => ({
        documents: s.documents.some((d) => d.id === id)
          ? s.documents.map((d) => (d.id === id ? doc : d))
          : [...s.documents, doc],
        loading: false,
      }));
      return doc;
    } catch (err) {
      set({ error: (err as Error).message, loading: false });
      throw err;
    }
  },

  updateDocument: async (id: number, data: UpdateDocumentInput) => {
    set({ loading: true, error: null });
    try {
      const doc = await api.updateDocument(id, data);
      set((s) => ({
        documents: s.documents.map((d) => (d.id === id ? doc : d)),
        loading: false,
      }));
      return doc;
    } catch (err) {
      set({ error: (err as Error).message, loading: false });
      throw err;
    }
  },

  deleteDocument: async (id: number) => {
    set({ loading: true, error: null });
    try {
      await api.deleteDocument(id);
      set((s) => ({
        documents: s.documents.filter((d) => d.id !== id),
        activeDocumentId: s.activeDocumentId === id ? null : s.activeDocumentId,
        loading: false,
      }));
    } catch (err) {
      set({ error: (err as Error).message, loading: false });
      throw err;
    }
  },

  setActiveDocument: (id: number | null) => {
    set({ activeDocumentId: id });
  },

  handleWSDocEvent: (event: WSMessage) => {
    const { document_id, user_id, type, content } = event;
    if (!document_id) return;

    switch (type) {
      case 'doc_open':
        if (user_id && content) {
          set((s) => ({
            viewers: {
              ...s.viewers,
              [document_id]: [
                ...(s.viewers[document_id] || []).filter((v) => v.userId !== user_id),
                { userId: user_id, username: content, connectedAt: Date.now() },
              ],
            },
          }));
        }
        break;

      case 'doc_close':
        if (user_id) {
          set((s) => ({
            viewers: {
              ...s.viewers,
              [document_id]: (s.viewers[document_id] || []).filter((v) => v.userId !== user_id),
            },
          }));
        }
        break;

      case 'doc_edit':
        // Another user edited the document - show editor indicator
        if (user_id && content) {
          set((s) => ({
            editors: {
              ...s.editors,
              [document_id]: content, // Store username of editor
            },
          }));
          // Clear editor indicator after 3 seconds
          setTimeout(() => {
            set((s) => {
              const updated = { ...s.editors };
              delete updated[document_id];
              return { editors: updated };
            });
          }, 3000);
        }
        break;
    }
  },

  clearViewers: (documentId: number) => {
    set((s) => {
      const updated = { ...s.viewers };
      delete updated[documentId];
      return { viewers: updated };
    });
  },

  clearEditors: (documentId: number) => {
    set((s) => {
      const updated = { ...s.editors };
      delete updated[documentId];
      return { editors: updated };
    });
  },
}));
