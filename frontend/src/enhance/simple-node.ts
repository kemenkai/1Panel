import { buildSimpleNodeVisitURL } from '@/enhance/api';

export const openSimpleNodePage = async (id: number, redirect = '/') => {
    const visitWindow = window.open('', '_blank');
    try {
        const res = await buildSimpleNodeVisitURL(id, redirect);
        if (!res?.data) {
            throw new Error('simple node visit url is empty');
        }
        if (visitWindow) {
            visitWindow.opener = null;
            visitWindow.location.replace(res.data);
            return;
        }
        window.open(res.data, '_blank', 'noopener,noreferrer');
    } catch (error) {
        visitWindow?.close();
        throw error;
    }
};
