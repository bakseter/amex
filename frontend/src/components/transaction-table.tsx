import { useEffect, useRef } from 'react';

import type { Meta, Transaction } from '@/api/transaction';
import TransactionRow from '@/components/transaction-row';

interface Props {
    transactions: Transaction[];
    selectedId: string | null;
    onSelect: (id: string) => void;
    meta: Meta;
}

const TransactionTable = ({
    transactions,
    selectedId,
    onSelect,
    meta,
}: Props) => {
    const selectedRef = useRef(null);

    useEffect(() => {
        selectedRef.current?.scrollIntoView({ block: 'nearest' });
    }, [selectedId]);

    return (
        <div className="overflow-y-auto flex-1">
            <table className="w-full border-collapse">
                <thead className="sticky top-0 z-10 bg-(--color-bg)">
                    <tr className="border-b border-(--color-border)">
                        {[
                            'Date',
                            'Description',
                            'Category',
                            'Cardholder',
                            'Amount',
                            '↔',
                        ].map((h, i) => (
                            <th
                                key={h}
                                className="px-3 py-2 text-left text-[10px] tracking-[.1em] uppercase text-(--color-muted) font-normal whitespace-nowrap"
                                style={
                                    i === 4
                                        ? { textAlign: 'right' }
                                        : i === 5
                                          ? { textAlign: 'center', width: 36 }
                                          : {}
                                }
                            >
                                {h}
                            </th>
                        ))}
                    </tr>
                </thead>
                <tbody>
                    {transactions.map((transaction) => (
                        <TransactionRow
                            key={transaction.id}
                            transaction={transaction}
                            selected={String(transaction.id) === selectedId}
                            onSelect={() => {
                                onSelect(String(transaction.id));
                            }}
                            meta={meta}
                            ref={
                                String(transaction.id) === selectedId
                                    ? selectedRef
                                    : null
                            }
                        />
                    ))}
                </tbody>
            </table>
        </div>
    );
};

export default TransactionTable;
