// components/transaction-table.tsx
import { useMemo, useState } from 'react';

import { useGetMetaQuery, useGetTransactionsQuery } from '@/api/transaction';
import TransactionRow from '@/components/transaction-row';
import { formatNumber, splitImpact } from '@/utils/utils';

type Lens = 'all' | 'uncategorized' | 'review' | 'edited';

const TransactionTable = ({ invoiceId }: { invoiceId: number }) => {
    const { data: transactions = [], isLoading } =
        useGetTransactionsQuery(invoiceId);
    const { data: meta } = useGetMetaQuery();

    const [query, setQuery] = useState('');
    const [holder, setHolder] = useState('');
    const [lens, setLens] = useState<Lens>('all');

    const persons = useMemo(() => {
        const keys = new Set<string>();

        transactions.forEach((transaction) => {
            Object.keys(transaction.owes).forEach((key) => {
                keys.add(key);
            });
        });

        return [...keys];
    }, [transactions]);

    // Rows whose split is still whatever the category guessed, and where being
    // wrong costs real money. Same rule the agent's review_splits uses.
    const reviewThreshold = 100;

    const counts = useMemo(
        () => ({
            uncategorized: transactions.filter(
                (transaction) => transaction.category === 'Uncategorized'
            ).length,
            review: transactions.filter(
                (transaction) =>
                    !transaction.splitLocked &&
                    splitImpact(transaction, meta) >= reviewThreshold
            ).length,
            edited: transactions.filter((transaction) => transaction.modified)
                .length,
        }),
        [transactions, meta]
    );

    const visible = useMemo(() => {
        const needle = query.toLowerCase();

        const rows = transactions.filter((transaction) => {
            if (
                needle &&
                !transaction.description.toLowerCase().includes(needle) &&
                !transaction.category.toLowerCase().includes(needle)
            ) {
                return false;
            }

            if (holder && transaction.cardholder !== holder) {
                return false;
            }

            if (
                lens === 'uncategorized' &&
                transaction.category !== 'Uncategorized'
            ) {
                return false;
            }

            if (
                lens === 'review' &&
                (transaction.splitLocked ||
                    splitImpact(transaction, meta) < reviewThreshold)
            ) {
                return false;
            }

            if (lens === 'edited' && !transaction.modified) {
                return false;
            }

            return true;
        });

        // In the review lens the useful order is by money at stake, not date.
        return lens === 'review'
            ? [...rows].sort(
                  (a, b) => splitImpact(b, meta) - splitImpact(a, meta)
              )
            : rows;
    }, [transactions, query, holder, lens, meta]);

    const visibleTotal = visible.reduce(
        (sum, transaction) => sum + transaction.amount,
        0
    );

    if (isLoading) {
        return <div className="p-6 text-muted2">Loading transactions…</div>;
    }

    const lenses: { key: Lens; label: string; count?: number }[] = [
        { key: 'all', label: 'All' },
        {
            key: 'uncategorized',
            label: 'Uncategorized',
            count: counts.uncategorized,
        },
        { key: 'review', label: 'Worth checking', count: counts.review },
        { key: 'edited', label: 'Edited', count: counts.edited },
    ];

    return (
        <div className="flex h-full flex-col overflow-hidden">
            <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-border px-5 py-2.5">
                <input
                    placeholder="Search merchants"
                    value={query}
                    onChange={(event) => {
                        setQuery(event.target.value);
                    }}
                    className="w-52 rounded-lg border border-border2 px-3 py-1.5 placeholder:text-muted focus:border-andreas focus:outline-none"
                />

                <select
                    value={holder}
                    onChange={(event) => {
                        setHolder(event.target.value);
                    }}
                    className="cursor-pointer rounded-lg border border-border2 px-2.5 py-1.5 focus:border-andreas focus:outline-none"
                >
                    <option value="">Both cards</option>
                    {meta?.cardholders.map((name) => (
                        <option key={name} value={name}>
                            {name}
                        </option>
                    ))}
                </select>

                <div className="flex gap-0.5">
                    {lenses.map((item) => (
                        <button
                            key={item.key}
                            onClick={() => {
                                setLens(item.key);
                            }}
                            className={`rounded-lg px-2.5 py-1.5 text-sm transition-colors ${
                                lens === item.key
                                    ? 'bg-surface2'
                                    : 'text-muted2 hover:bg-surface'
                            }`}
                        >
                            {item.label}
                            {item.count !== undefined && item.count > 0 && (
                                <span className="ml-1.5 tabular-nums text-muted">
                                    {item.count}
                                </span>
                            )}
                        </button>
                    ))}
                </div>

                <div className="ml-auto text-xs tabular-nums text-muted2">
                    {visible.length === transactions.length
                        ? `${String(transactions.length)} rows`
                        : `${String(visible.length)} of ${String(transactions.length)}`}
                    {' · '}
                    {formatNumber(visibleTotal)}
                </div>
            </div>

            <div className="flex-1 overflow-auto">
                <table className="w-full min-w-[780px] border-collapse text-left">
                    <thead className="sticky top-0 bg-bg text-xs text-muted2">
                        <tr className="border-b border-border">
                            <th className="py-2 pr-3 pl-5 font-medium">Date</th>
                            <th className="py-2 pr-3 font-medium">Merchant</th>
                            <th className="py-2 pr-3 font-medium">Category</th>
                            <th className="py-2 pr-3 font-medium">Card</th>
                            <th className="py-2 pr-3 font-medium">Split</th>
                            <th className="py-2 pr-3 text-right font-medium">
                                Amount
                            </th>
                            {persons.map((person) => (
                                <th
                                    key={person}
                                    className="py-2 pr-5 text-right font-medium"
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
                                meta={meta}
                                persons={persons}
                            />
                        ))}
                    </tbody>
                </table>

                {visible.length === 0 && (
                    <div className="py-16 text-center text-muted2">
                        {lens === 'uncategorized'
                            ? 'Everything has a category.'
                            : lens === 'review'
                              ? 'No unreviewed split moves more than 100 NOK.'
                              : 'Nothing matches those filters.'}
                    </div>
                )}
            </div>
        </div>
    );
};

export default TransactionTable;
