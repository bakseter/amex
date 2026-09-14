// api/transaction.ts
import { createApi, fetchBaseQuery } from '@reduxjs/toolkit/query/react';

const envUrl = import.meta.env.VITE_BACKEND_URL as string | undefined;

// Trailing slashes are trimmed so callers can safely template onto this.
const backendUrl = (envUrl && envUrl.length > 0 ? envUrl : '/api').replace(
    /\/+$/,
    ''
);

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
    /** Split was decided deliberately and survives rule runs. */
    splitLocked: boolean;
    splitReason: string;
    city: string;
    country: string;
    original: TransactionOriginal;
}

export interface TransactionPatch {
    isShared?: boolean;
    category?: string;
    cardholder?: string;
    splitReason?: string;
    /** Unlock the split and fall back to the category default. */
    resetSplit?: boolean;
}

export interface Invoice {
    id: number;
    filename: string;
    uploaded_at: string;
    transaction_count: number;
}

/** POST /invoices returns invoice_id, not id. */
export interface UploadResult {
    invoice_id: number;
    filename: string;
    uploaded_at: string;
    transaction_count: number;
}

export interface Meta {
    categories: string[];
    cardholders: string[];
    sharedCategories: string[];
    splitRatio: Record<string, number>;
}

export interface Summary {
    byCategory: [string, Record<string, number> & { total: number }][];
    personTotals: Record<string, number>;
    grandTotal: number;
    persons: string[];
}

const blobUrl = async (response: Response) =>
    URL.createObjectURL(await response.blob());

// ── API ────────────────────────────────────────────────────────────────────────

const api = createApi({
    reducerPath: 'api',
    baseQuery: fetchBaseQuery({ baseUrl: `${backendUrl}/` }),
    tagTypes: ['Invoice', 'Transaction', 'Meta', 'Summary'],
    endpoints: (builder) => ({
        // ── Invoices ───────────────────────────────────────────────────────────

        listInvoices: builder.query<Invoice[], void>({
            query: () => 'invoices',
            providesTags: ['Invoice'],
        }),

        uploadInvoice: builder.mutation<UploadResult, File>({
            query: (file) => {
                const body = new FormData();
                body.append('file', file);

                return { url: 'invoices', method: 'POST', body };
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

        // Returns a blob URL — revoke it with URL.revokeObjectURL when done
        downloadInvoiceCsv: builder.query<string, number>({
            query: (invoiceId) => ({
                url: `invoices/${String(invoiceId)}/csv`,
                responseHandler: blobUrl,
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
            // Update the row in place so the table doesn't flash while the
            // summary refetches.
            async onQueryStarted(
                { invoiceId, id, fields },
                { dispatch, queryFulfilled }
            ) {
                const patchResult = dispatch(
                    api.util.updateQueryData(
                        'getTransactions',
                        invoiceId,
                        (draft) => {
                            const row = draft.find(
                                (transaction) => transaction.id === id
                            );

                            if (row) {
                                Object.assign(row, fields);
                            }
                        }
                    )
                );

                try {
                    const { data } = await queryFulfilled;

                    dispatch(
                        api.util.updateQueryData(
                            'getTransactions',
                            invoiceId,
                            (draft) => {
                                const index = draft.findIndex(
                                    (transaction) => transaction.id === id
                                );

                                if (index !== -1) {
                                    draft[index] = data;
                                }
                            }
                        )
                    );
                } catch {
                    patchResult.undo();
                }
            },
            invalidatesTags: (_result, _error, { invoiceId }) => [
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
    useDownloadInvoiceCsvQuery,
    useGetTransactionsQuery,
    usePatchTransactionMutation,
    useGetSummaryQuery,
    useGetMetaQuery,
} = api;

export { backendUrl };

export default api;
