import { createApi, fetchBaseQuery } from '@reduxjs/toolkit/query/react';

const backendUrl = 'http://localhost:8000';

interface Transaction {
    id: number;
    date: string;
    description: string;
    amount: number;
    cardholder: string;
    category: string;
    isShared: boolean;
    owes: number;
}

interface Meta {
    categories: string[];
    cardholders: string[];
    sharedCategories: string[];
    pdf: string;
}

interface Summary {
    byCategory: { category: string; total: number }[];
    personTotals: Record<string, number>;
    grandTotal: number;
    persons: string[];
}

const api = createApi({
    baseQuery: fetchBaseQuery({
        baseUrl: `${backendUrl}/api/`,
    }),
    tagTypes: ['Transaction', 'Meta', 'Summary'],
    endpoints: (builder) => ({
        getTransactions: builder.query<Transaction[], void>({
            query: () => 'transactions',
            providesTags: ['Transaction'],
        }),
        patchTransaction: builder.mutation<
            Transaction,
            { id: number; fields: Partial<Transaction> }
        >({
            query: ({ id, fields }) => ({
                url: `transactions/${String(id)}`,
                method: 'PATCH',
                body: fields,
            }),
            invalidatesTags: ['Transaction'],
        }),
        exportCsv: builder.mutation<string, void>({
            query: () => ({
                url: 'export',
                method: 'POST',
            }),
        }),
        getMeta: builder.query<Meta, void>({
            query: () => 'meta',
            providesTags: ['Meta'],
        }),
        getSummary: builder.query<Summary, void>({
            query: () => 'summary',
            providesTags: ['Summary'],
        }),
    }),
});

export type { Meta, Summary, Transaction };

export const {
    useGetTransactionsQuery,
    usePatchTransactionMutation,
    useExportCsvMutation,
    useGetMetaQuery,
    useGetSummaryQuery,
} = api;

export default api;
