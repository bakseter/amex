// components/transaction-row.tsx
import { useState } from 'react';

import {
    type Meta,
    type Transaction,
    type TransactionPatch,
    usePatchTransactionMutation,
} from '@/api/transaction';
import EditSelect from '@/components/edit-select';
import { formatNumber, personColor, splitImpact } from '@/utils/utils';

const TransactionRow = ({
    transaction,
    meta,
    persons,
}: {
    transaction: Transaction;
    meta: Meta | undefined;
    persons: string[];
}) => {
    const [patch] = usePatchTransactionMutation();
    const [editingReason, setEditingReason] = useState(false);

    const update = (fields: TransactionPatch) => {
        void patch({
            invoiceId: transaction.invoice_id,
            id: transaction.id,
            fields,
        });
    };

    const holderColor = personColor(transaction.cardholder, persons);
    const uncategorized = transaction.category === 'Uncategorized';
    const impact = splitImpact(transaction, meta);
    const abroad =
        transaction.country !== '' &&
        transaction.country.toUpperCase() !== 'NORWAY';

    return (
        <>
            <tr className="tx-row border-b border-border hover:bg-surface">
                <td className="py-2.5 pr-3 pl-5 whitespace-nowrap text-muted2">
                    {transaction.date}
                </td>
                <td className="max-w-[260px] py-2.5 pr-3">
                    <span
                        className="block truncate"
                        title={transaction.description}
                    >
                        {transaction.description}
                    </span>
                    {(transaction.modified || abroad) && (
                        <span className="text-xs text-muted">
                            {transaction.modified && 'edited'}
                            {transaction.modified && abroad && ' · '}
                            {abroad && transaction.country.toLowerCase()}
                        </span>
                    )}
                </td>
                <td className="py-2.5 pr-3">
                    <EditSelect
                        value={transaction.category}
                        options={meta?.categories ?? []}
                        tone={uncategorized ? 'attention' : 'default'}
                        onChange={(category) => {
                            update({ category });
                        }}
                    />
                </td>
                <td className="py-2.5 pr-3">
                    <EditSelect
                        value={transaction.cardholder}
                        options={meta?.cardholders ?? []}
                        color={holderColor}
                        onChange={(cardholder) => {
                            update({ cardholder });
                        }}
                    />
                </td>
                <td className="py-2.5 pr-3">
                    <div className="flex items-center gap-1.5">
                        <button
                            onClick={() => {
                                update({ isShared: !transaction.isShared });
                            }}
                            title={
                                transaction.isShared
                                    ? `Split — flipping this moves ${formatNumber(impact)} NOK`
                                    : `${transaction.cardholder} alone — flipping this moves ${formatNumber(impact)} NOK`
                            }
                            className={`rounded-md border px-2 py-0.5 text-xs transition-colors ${
                                transaction.isShared
                                    ? 'border-shared/30 bg-shared/10 text-shared'
                                    : 'border-border2 text-muted2 hover:bg-surface2'
                            }`}
                        >
                            {transaction.isShared ? 'Split' : 'Solo'}
                        </button>
                        {transaction.splitLocked && (
                            <button
                                onClick={() => {
                                    setEditingReason((open) => !open);
                                }}
                                title={
                                    transaction.splitReason ||
                                    'Decided deliberately — rule runs leave it alone'
                                }
                                className="text-xs text-muted hover:text-text"
                            >
                                ●
                            </button>
                        )}
                    </div>
                </td>
                <td className="py-2.5 pr-3 text-right tabular-nums">
                    {formatNumber(transaction.amount)}
                </td>
                {persons.map((person) => {
                    const owed = transaction.owes[person] ?? 0;

                    return (
                        <td
                            key={person}
                            className="py-2.5 pr-5 text-right tabular-nums"
                            style={{
                                color:
                                    owed > 0
                                        ? personColor(person, persons)
                                        : 'var(--color-muted)',
                            }}
                        >
                            {owed > 0 ? formatNumber(owed) : '–'}
                        </td>
                    );
                })}
            </tr>

            {editingReason && (
                <tr className="border-b border-border bg-surface">
                    <td colSpan={6 + persons.length} className="px-5 py-2">
                        <div className="flex items-center gap-2">
                            <input
                                autoFocus
                                defaultValue={transaction.splitReason}
                                placeholder="Why this split? e.g. solo work trip"
                                onKeyDown={(event) => {
                                    if (event.key === 'Enter') {
                                        update({
                                            splitReason:
                                                event.currentTarget.value,
                                        });
                                        setEditingReason(false);
                                    }

                                    if (event.key === 'Escape') {
                                        setEditingReason(false);
                                    }
                                }}
                                className="flex-1 rounded-md border border-border2 bg-bg px-2.5 py-1.5 placeholder:text-muted focus:border-andreas focus:outline-none"
                            />
                            <button
                                onClick={() => {
                                    update({ resetSplit: true });
                                    setEditingReason(false);
                                }}
                                className="rounded-md px-2 py-1.5 text-xs text-muted2 hover:text-danger"
                                title="Unlock and follow the category default again"
                            >
                                Reset
                            </button>
                        </div>
                    </td>
                </tr>
            )}
        </>
    );
};

export default TransactionRow;
