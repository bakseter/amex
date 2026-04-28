import { createApi, fetchBaseQuery } from '@reduxjs/toolkit/query/react';

const backendUrl =
    (import.meta.env.VITE_BACKEND_URL as string | undefined | null) ??
    'http://localhost:8000/api';

// ── Types ──────────────────────────────────────────────────────────────────────

export interface TransactionOriginal {
    category: string;
    isShared: boolean;
    cardholder: string;
}

export interface Transaction {
    id: number;
    invoice_id: number;
    date: string;
    description: string;
    amount: number;
    cardholder: string;
    category: string;
    isShared: boolean;
    owes: Record<string, number>;
    modified: boolean;
    original: TransactionOriginal;
}

export interface TransactionPatch {
    isShared?: boolean;
    category?: string;
    cardholder?: string;
}

export interface Invoice {
    id: number;
    filename: string;
    uploaded_at: string;
    transaction_count: number;
}

export interface Meta {
    categories: string[];
    cardholders: string[];
    sharedCategories: string[];
}

export interface Summary {
    byCategory: [string, Record<string, number> & { total: number }][];
    personTotals: Record<string, number>;
    grandTotal: number;
    persons: string[];
}

// ── API ────────────────────────────────────────────────────────────────────────

const api = createApi({
    reducerPath: 'api',
    baseQuery: fetchBaseQuery({
        baseUrl: `${backendUrl}/`,
    }),
    tagTypes: ['Invoice', 'Transaction', 'Meta', 'Summary'],
    endpoints: (builder) => ({
        // ── Invoices ───────────────────────────────────────────────────────────

        listInvoices: builder.query<Invoice[], void>({
            query: () => 'invoices',
            providesTags: ['Invoice'],
        }),

        uploadInvoice: builder.mutation<Invoice, File>({
            query: (file) => {
                const body = new FormData();
                body.append('file', file);

                return {
                    url: 'invoices',
                    method: 'POST',
                    body,
                };
            },
            invalidatesTags: ['Invoice'],
        }),

        deleteInvoice: builder.mutation<void, number>({
            query: (invoiceId) => ({
                url: `invoices/${String(invoiceId)}`,
                method: 'DELETE',
            }),
            invalidatesTags: ['Invoice'],
        }),

        // Returns a blob URL — call URL.createObjectURL on the result
        downloadInvoicePdf: builder.query<string, number>({
            query: (invoiceId) => ({
                url: `invoices/${String(invoiceId)}/pdf`,
                responseHandler: async (response) => {
                    const blob = await response.blob();

                    return URL.createObjectURL(blob);
                },
                // Prevent RTK Query from trying to parse as JSON
                cache: 'no-cache',
            }),
        }),

        // ── Transactions ───────────────────────────────────────────────────────

        getTransactions: builder.query<Transaction[], number>({
            query: (invoiceId) => `invoices/${String(invoiceId)}/transactions`,
            providesTags: (_result, _error, invoiceId) => [
                { type: 'Transaction', id: invoiceId },
            ],
        }),

        patchTransaction: builder.mutation<
            Transaction,
            { invoiceId: number; id: number; fields: TransactionPatch }
        >({
            query: ({ id, fields }) => ({
                url: `transactions/${String(id)}`,
                method: 'PATCH',
                body: fields,
            }),
            invalidatesTags: (_result, _error, { invoiceId }) => [
                { type: 'Transaction', id: invoiceId },
                { type: 'Summary', id: invoiceId },
            ],
        }),

        // ── Summary ────────────────────────────────────────────────────────────

        getSummary: builder.query<Summary, number>({
            query: (invoiceId) => `invoices/${String(invoiceId)}/summary`,
            providesTags: (_result, _error, invoiceId) => [
                { type: 'Summary', id: invoiceId },
            ],
        }),

        // ── Export ─────────────────────────────────────────────────────────────

        // Both export endpoints return a blob URL for download
        exportTransactionsCsv: builder.query<string, number>({
            query: (invoiceId) => ({
                url: `invoices/${String(invoiceId)}/export/transactions`,
                responseHandler: async (response) => {
                    const blob = await response.blob();

                    return URL.createObjectURL(blob);
                },
                cache: 'no-cache',
            }),
        }),

        exportSummaryCsv: builder.query<string, number>({
            query: (invoiceId) => ({
                url: `invoices/${String(invoiceId)}/export/summary`,
                responseHandler: async (response) => {
                    const blob = await response.blob();

                    return URL.createObjectURL(blob);
                },
                cache: 'no-cache',
            }),
        }),

        // ── Meta ───────────────────────────────────────────────────────────────

        getMeta: builder.query<Meta, void>({
            query: () => 'meta',
            providesTags: ['Meta'],
        }),
    }),
});

export const {
    useListInvoicesQuery,
    useUploadInvoiceMutation,
    useDeleteInvoiceMutation,
    useDownloadInvoicePdfQuery,
    useGetTransactionsQuery,
    usePatchTransactionMutation,
    useGetSummaryQuery,
    useExportTransactionsCsvQuery,
    useExportSummaryCsvQuery,
    useGetMetaQuery,
} = api;

export { backendUrl };

export default api;
