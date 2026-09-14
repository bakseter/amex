// utils/utils.ts
import type { Meta, Transaction } from '@/api/transaction';

const nok = new Intl.NumberFormat('nb-NO', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
});

const formatNumber = (num: number) => nok.format(num);

/**
 * Each person gets a fixed colour, assigned by their position in the cardholder
 * list rather than by name, so the palette holds if the names ever change.
 */
const PERSON_COLORS = ['var(--color-andreas)', 'var(--color-nikoline)'];

const personColor = (name: string, people: string[]) => {
    const index = people.indexOf(name);

    if (index === -1) {
        return 'var(--color-muted)';
    }

    return PERSON_COLORS[index % PERSON_COLORS.length];
};

/** Statement dates arrive as dd.mm.yy. */
const parseStatementDate = (date: string) => {
    const [day, month, year] = date.split('.');

    if (!day || !month || !year) {
        return null;
    }

    return new Date(2000 + Number(year), Number(month) - 1, Number(day));
};

const monthLabel = (transactions: Transaction[]) => {
    const dates = transactions
        .map((transaction) => parseStatementDate(transaction.date))
        .filter((date): date is Date => date !== null)
        .sort((a, b) => a.getTime() - b.getTime());

    const first = dates[0];
    const last = dates.at(-1);

    if (!first || !last) {
        return '';
    }

    const format = (date: Date) =>
        new Intl.DateTimeFormat('en-GB', {
            month: 'short',
            year: 'numeric',
        }).format(date);

    return format(first) === format(last)
        ? format(first)
        : `${format(first)} – ${format(last)}`;
};

/**
 * How much the balance moves if this transaction's split flips. This is the
 * only number that says whether a row is worth arguing about.
 */
const splitImpact = (transaction: Transaction, meta: Meta | undefined) => {
    const ratio = meta?.splitRatio[transaction.cardholder] ?? 0.5;

    return transaction.amount * (1 - ratio);
};

export {
    formatNumber,
    monthLabel,
    parseStatementDate,
    personColor,
    splitImpact,
};
