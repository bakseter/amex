const formatNumber = (num: number) =>
    new Intl.NumberFormat('nb-NO', {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
    }).format(num);

const holderColor = (name: string) => {
    if (!name) {
        return 'var(--color-muted)';
    }
    return name.toLowerCase().startsWith('a')
        ? 'var(--color-andreas)'
        : 'var(--color-nikoline)';
};

export { formatNumber, holderColor };
