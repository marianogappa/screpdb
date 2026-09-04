export const formatLoadingShortWith = (translate) => translate('library.loadingShort');

export const stillLoadingCopyWith = (translate) => translate('library.stillLoading');

export const isLibraryLoading = (status) => String(status || '').toLowerCase() === 'loading';
