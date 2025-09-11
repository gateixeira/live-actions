const paginationUtils = {
    initializeHandlers(config) {
        const pageSizeSelect = document.getElementById(config.pageSizeId);
        pageSizeSelect?.addEventListener('change', (e) => {
            config.loadFunction(1, parseInt(e.target.value));
        });
        
        const handlers = {
            [config.firstId]: () => config.loadFunction(1),
            [config.prevId]: () => config.loadFunction(config.paginationState.currentPage - 1),
            [config.nextId]: () => config.loadFunction(config.paginationState.currentPage + 1),
            [config.lastId]: () => config.loadFunction(config.paginationState.totalPages)
        };
        
        Object.entries(handlers).forEach(([id, handler]) => {
            const button = document.getElementById(id);
            button?.addEventListener('click', handler);
        });
    },
    
    updateControls(config) {
        this.updateInfo(config);
        this.updateButtons(config);
        this.updatePageNumbers(config);
    },
    
    updateInfo(config) {
        const paginationInfo = document.getElementById(config.infoId);
        if (!paginationInfo) return;
        
        const { currentPage, currentPageSize, totalCount } = config.paginationState;
        const startItem = (currentPage - 1) * currentPageSize + 1;
        const endItem = Math.min(currentPage * currentPageSize, totalCount);
        
        paginationInfo.textContent = totalCount === 0 ? 
            "No items found" : 
            `Showing ${startItem}-${endItem} of ${totalCount} ${config.itemName}`;
    },
    
    updateButtons(config) {
        const buttons = {
            [config.firstId]: document.getElementById(config.firstId),
            [config.prevId]: document.getElementById(config.prevId),
            [config.nextId]: document.getElementById(config.nextId),
            [config.lastId]: document.getElementById(config.lastId)
        };
        
        const { currentPage, totalPages } = config.paginationState;
        
        if (buttons[config.firstId]) buttons[config.firstId].disabled = currentPage === 1;
        if (buttons[config.prevId]) buttons[config.prevId].disabled = currentPage === 1;
        if (buttons[config.nextId]) buttons[config.nextId].disabled = currentPage === totalPages;
        if (buttons[config.lastId]) buttons[config.lastId].disabled = currentPage === totalPages;
    },
    
    updatePageNumbers(config) {
        const pageNumbers = document.getElementById(config.pageNumbersId);
        if (!pageNumbers) return;
        
        pageNumbers.innerHTML = '';
        
        const { currentPage, totalPages } = config.paginationState;
        const { MAX_VISIBLE_PAGES, SIDE_PAGES } = CONFIG.PAGINATION;
        
        if (totalPages <= 1) return;
        
        let startPage = Math.max(1, currentPage - SIDE_PAGES);
        let endPage = Math.min(totalPages, currentPage + SIDE_PAGES);
        
        if (currentPage <= SIDE_PAGES + 1) {
            endPage = Math.min(totalPages, MAX_VISIBLE_PAGES);
        } else if (currentPage >= totalPages - SIDE_PAGES) {
            startPage = Math.max(1, totalPages - MAX_VISIBLE_PAGES + 1);
        }
        
        // First page
        if (startPage > 1) {
            pageNumbers.appendChild(this.createPageButton(1, config));
            if (startPage > 2) {
                pageNumbers.appendChild(this.createEllipsis());
            }
        }
        
        // Page range
        for (let i = startPage; i <= endPage; i++) {
            pageNumbers.appendChild(this.createPageButton(i, config));
        }
        
        // Last page
        if (endPage < totalPages) {
            if (endPage < totalPages - 1) {
                pageNumbers.appendChild(this.createEllipsis());
            }
            pageNumbers.appendChild(this.createPageButton(totalPages, config));
        }
    },
    
    createPageButton(pageNum, config) {
        const button = document.createElement('button');
        button.className = `page-number ${pageNum === config.paginationState.currentPage ? 'active' : ''}`;
        button.textContent = pageNum;
        button.addEventListener('click', () => {
            if (pageNum !== config.paginationState.currentPage) {
                config.loadFunction(pageNum);
            }
        });
        return button;
    },
    
    createEllipsis() {
        const span = document.createElement('span');
        span.className = 'page-ellipsis';
        span.textContent = '…';
        return span;
    }
};