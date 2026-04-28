import {
    type Transaction,
    type TransactionPatch,
    usePatchTransactionMutation,
} from '@/api/transaction';
import EditSelect from '@/components/edit-select';

const TransactionRow = ({
    transaction,
    categories,
    cardholders,
    persons,
}: {
    transaction: Transaction;
    categories: string[];
    cardholders: string[];
    persons: string[];
}) => {
    const [patch] = usePatchTransactionMutation();

    const update = (fields: TransactionPatch) =>
        patch({
            invoiceId: transaction.invoice_id,
            id: transaction.id,
            fields,
        });

    return (
        <tr
            className={`border-b border-gray-100 text-sm ${transaction.modified ? 'bg-amber-50' : 'hover:bg-gray-50'}`}
        >
            <td className="px-3 py-2 text-gray-400 text-xs whitespace-nowrap">
                {transaction.date}
            </td>
            <td className="px-3 py-2 max-w-[220px]">
                <span
                    className="truncate block"
                    title={transaction.description}
                >
                    {transaction.description}
                </span>
                {transaction.modified && (
                    <span className="text-[10px] text-amber-500 font-medium">
                        edited
                    </span>
                )}
            </td>
            <td className="px-3 py-2">
                {/* eslint-disable @typescript-eslint/no-misused-promises */}
                <EditSelect
                    value={transaction.category}
                    options={categories}
                    onChange={(category) => update({ category })}
                />
                {/* eslint-enable @typescript-eslint/no-misused-promises */}
            </td>
            <td className="px-3 py-2">
                {/* eslint-disable @typescript-eslint/no-misused-promises */}
                <EditSelect
                    value={transaction.cardholder}
                    options={cardholders}
                    onChange={(cardholder) => update({ cardholder })}
                />
                {/* eslint-enable @typescript-eslint/no-misused-promises */}
            </td>
            <td className="px-3 py-2 text-center">
                {/* eslint-disable @typescript-eslint/no-misused-promises */}
                <input
                    type="checkbox"
                    checked={transaction.isShared}
                    onChange={(event) =>
                        update({ isShared: event.target.checked })
                    }
                    onClick={(event) => {
                        event.stopPropagation();
                    }}
                    className="accent-blue-500"
                />
                {/* eslint-enable @typescript-eslint/no-misused-promises */}
            </td>
            <td className="px-3 py-2 text-right font-mono text-xs">
                {transaction.amount.toFixed(2)}
            </td>
            {persons.map((person) => (
                <td
                    key={person}
                    className="px-3 py-2 text-right font-mono text-xs text-gray-500"
                >
                    {(transaction.owes[person] ?? 0).toFixed(2)}
                </td>
            ))}
        </tr>
    );
};

export default TransactionRow;
