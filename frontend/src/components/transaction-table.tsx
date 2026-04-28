import { useMemo, useState } from 'react';

import { useGetMetaQuery, useGetTransactionsQuery } from '@/api/transaction';
import TransactionRow from '@/components/transaction-row';

const TransactionTable = ({ invoiceId }: { invoiceId: number }) => {
    const { data: transactions = [], isLoading } =
        useGetTransactionsQuery(invoiceId);
    const { data: meta } = useGetMetaQuery();

    const [filters, setFilters] = useState({
        query: '',
        holder: '',
        category: '',
        modified: false,
    });

    const visible = useMemo(
        () =>
            transactions.filter((transaction) => {
                if (
                    filters.query &&
                    !transaction.description
                        .toLowerCase()
                        .includes(filters.query.toLowerCase()) &&
                    !transaction.category
                        .toLowerCase()
                        .includes(filters.query.toLowerCase())
                ) {
                    return false;
                }

                if (
                    filters.holder &&
                    transaction.cardholder !== filters.holder
                ) {
                    return false;
                }

                if (
                    filters.category &&
                    transaction.category !== filters.category
                ) {
                    return false;
                }

                if (filters.modified && !transaction.modified) {
                    return false;
                }

                return true;
            }),
        [transactions, filters]
    );

    const persons = useMemo(() => {
        const keys = new Set<string>();

        transactions.forEach((transaction) => {
            Object.keys(transaction.owes).forEach((key) => keys.add(key));
        });

        return [...keys];
    }, [transactions]);

    if (isLoading) {
        return (
            <div className="p-6 text-sm text-gray-400">
                Loading transactions…
            </div>
        );
    }

    return (
        <div className="flex flex-col h-full overflow-hidden">
            {/* Filters */}
            <div className="flex gap-2 px-3 py-2 border-b border-gray-100 text-sm shrink-0 flex-wrap">
                <input
                    placeholder="Search…"
                    value={filters.query}
                    onChange={(event) => {
                        setFilters((filter) => ({
                            ...filter,
                            query: event.target.value,
                        }));
                    }}
                    className="border border-gray-200 rounded px-2 py-1 text-xs focus:outline-none focus:border-blue-400 w-44"
                />
                <select
                    value={filters.holder}
                    onChange={(event) => {
                        setFilters((filter) => ({
                            ...filter,
                            holder: event.target.value,
                        }));
                    }}
                    className="border border-gray-200 rounded px-2 py-1 text-xs focus:outline-none"
                >
                    <option value="">All cardholders</option>
                    {meta?.cardholders.map((holder) => (
                        <option key={holder}>{holder}</option>
                    ))}
                </select>
                <select
                    value={filters.category}
                    onChange={(event) => {
                        setFilters((filter) => ({
                            ...filter,
                            category: event.target.value,
                        }));
                    }}
                    className="border border-gray-200 rounded px-2 py-1 text-xs focus:outline-none"
                >
                    <option value="">All categories</option>
                    {meta?.categories.map((category) => (
                        <option key={category}>{category}</option>
                    ))}
                </select>
                <label className="flex items-center gap-1 text-xs text-gray-500 cursor-pointer">
                    <input
                        type="checkbox"
                        checked={filters.modified}
                        onChange={(event) => {
                            setFilters((filter) => ({
                                ...filter,
                                modified: event.target.checked,
                            }));
                        }}
                        className="accent-amber-400"
                    />
                    Edited only
                </label>
                <span className="ml-auto text-xs text-gray-400 self-center">
                    {visible.length} / {transactions.length}
                </span>
            </div>

            {/* Table */}
            <div className="overflow-auto flex-1">
                <table className="w-full text-left border-collapse min-w-[700px]">
                    <thead className="sticky top-0 bg-white border-b border-gray-200 text-xs text-gray-400 uppercase tracking-wide">
                        <tr>
                            <th className="px-3 py-2">Date</th>
                            <th className="px-3 py-2">Description</th>
                            <th className="px-3 py-2">Category</th>
                            <th className="px-3 py-2">Cardholder</th>
                            <th className="px-3 py-2 text-center">Shared</th>
                            <th className="px-3 py-2 text-right">Amount</th>
                            {persons.map((person) => (
                                <th
                                    key={person}
                                    className="px-3 py-2 text-right"
                                >
                                    {person}
                                </th>
                            ))}
                        </tr>
                    </thead>
                    <tbody>
                        {visible.map((transaction) => (
                            <TransactionRow
                                key={transaction.id}
                                transaction={transaction}
                                categories={meta?.categories ?? []}
                                cardholders={meta?.cardholders ?? []}
                                persons={persons}
                            />
                        ))}
                    </tbody>
                </table>
                {visible.length === 0 && (
                    <div className="py-12 text-center text-sm text-gray-300">
                        No transactions match filters
                    </div>
                )}
            </div>
        </div>
    );
};

export default TransactionTable;
