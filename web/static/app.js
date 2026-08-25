document.addEventListener('alpine:init', () => {
  Alpine.data('adleadApp', () => ({
    // Tema (Color Mode: 'light' | 'dark' | 'system')
    themeMode: localStorage.getItem('themeMode') || 'system',

    // Estado de Busca
    searchTerms: '',
    limit: 25,
    adDeliveryDateMin: '',
    publisherPlatform: 'TODAS',
    onlyWhatsApp: false,
    onlyEmail: false,
    minScore: 0,

    // Personalização de Oferta / Pitch Suffix (persistido no localStorage)
    customPitch: localStorage.getItem('nexus_custom_pitch') || 'Somos especialistas em estruturar o Diagnóstico Comercial para empresas ajustarem gargalos e ganharem previsibilidade de vendas. Gostaria de entender: como está a taxa de conversão dos leads que chegam dos seus anúncios hoje?',
    showPitchEditor: false,
    
    // Modal de Visualização de Copy
    copyModalOpen: false,
    selectedLeadForCopy: null,
    
    // Estado de Execução
    loading: false,
    loadingStep: 'Iniciando busca...',
    
    // Dados de Leads e Estatísticas
    leads: [],
    totalCount: 0,
    stats: {
      total_leads: 0,
      new_leads: 0,
      contacted_leads: 0,
      discarded_leads: 0,
      high_potential_leads: 0,
      with_whatsapp: 0,
      with_email: 0
    },
    
    // Filtros Locais e Visualização
    currentTab: 'Todos',
    localSearch: '',
    expandedLeadId: null,
    
    // Notificações Toast
    toast: {
      show: false,
      message: '',
      type: 'success' // 'success' | 'error' | 'info'
    },

    async init() {
      this.initTheme();
      await this.loadStats();
      await this.loadLeads();
      
      // Auto-refresh de estatísticas a cada 30 segundos
      setInterval(() => {
        if (!this.loading) {
          this.loadStats();
        }
      }, 30000);
    },

    initTheme() {
      this.applyTheme();
      
      // Ouvinte para mudanças do sistema operacional
      window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
        if (this.themeMode === 'system') {
          this.applyTheme();
        }
      });
    },

    setTheme(mode) {
      this.themeMode = mode;
      localStorage.setItem('themeMode', mode);
      this.applyTheme();
      this.showToast(`Modo de cor alterado para: ${mode === 'system' ? 'Sistema' : mode === 'dark' ? 'Escuro' : 'Claro'}`, 'info');
    },

    applyTheme() {
      const isDark = this.themeMode === 'dark' || 
        (this.themeMode === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
      
      if (isDark) {
        document.documentElement.classList.add('dark');
      } else {
        document.documentElement.classList.remove('dark');
      }
    },

    saveCustomPitch() {
      localStorage.setItem('nexus_custom_pitch', this.customPitch);
      this.showToast('Pitch / Complemento da Oferta salvo com sucesso!', 'success');
    },

    resetCustomPitch() {
      this.customPitch = 'Somos especialistas em estruturar o Diagnóstico Comercial para empresas ajustarem gargalos e ganharem previsibilidade de vendas. Gostaria de entender: como está a taxa de conversão dos leads que chegam dos seus anúncios hoje?';
      localStorage.setItem('nexus_custom_pitch', this.customPitch);
      this.showToast('Pitch restaurado para a versão padrão', 'info');
    },

    getFullMessage(lead) {
      if (!lead) return '';
      const baseIcebreaker = (lead.ai_icebreaker || `Olá equipe da ${lead.company_name}! Vi os anúncios ativos de vocês no Meta Ads.`).trim();
      const pitch = (this.customPitch || '').trim();
      if (!pitch) return baseIcebreaker;
      return `${baseIcebreaker}\n\n${pitch}`;
    },

    openCopyModal(lead) {
      this.selectedLeadForCopy = lead;
      this.copyModalOpen = true;
    },

    closeCopyModal() {
      this.copyModalOpen = false;
      this.selectedLeadForCopy = null;
    },

    async loadStats() {
      try {
        const res = await fetch('/api/stats');
        if (res.ok) {
          this.stats = await res.json();
        }
      } catch (err) {
        console.error('Erro ao carregar estatísticas:', err);
      }
    },

    async loadLeads() {
      try {
        const params = new URLSearchParams();
        if (this.currentTab && this.currentTab !== 'Todos') {
          params.append('status', this.currentTab);
        }
        if (this.localSearch) {
          params.append('search', this.localSearch);
        }
        if (this.onlyWhatsApp) {
          params.append('only_whatsapp', 'true');
        }
        if (this.onlyEmail) {
          params.append('only_email', 'true');
        }
        if (this.minScore > 0) {
          params.append('min_score', this.minScore);
        }
        params.append('limit', '200');

        const res = await fetch(`/api/leads?${params.toString()}`);
        if (res.ok) {
          const data = await res.json();
          this.leads = data.leads || [];
          this.totalCount = data.total || 0;
          this.refreshIcons();
        }
      } catch (err) {
        console.error('Erro ao carregar leads:', err);
        this.showToast('Erro ao listar leads do banco de dados', 'error');
      }
    },

    refreshIcons() {
      this.$nextTick(() => {
        setTimeout(() => {
          if (window.lucide && typeof window.lucide.createIcons === 'function') {
            window.lucide.createIcons();
          }
        }, 50);
      });
    },

    async startSearch() {
      if (!this.searchTerms.trim()) {
        this.showToast('Por favor, digite um nicho ou palavra-chave para buscar.', 'error');
        return;
      }

      this.loading = true;
      this.loadingStep = 'Consultando Meta Ad Library API com paginação inteligente...';

      const stepTimer1 = setTimeout(() => {
        if (this.loading) this.loadingStep = 'Raspando Landing Pages concorrentemente (WhatsApp, E-mails)...';
      }, 3000);

      const stepTimer2 = setTimeout(() => {
        if (this.loading) this.loadingStep = 'Qualificando maturidade & nicho com Google Gemini 3.7 Flash...';
      }, 7000);

      try {
        const platforms = [];
        if (this.publisherPlatform && this.publisherPlatform !== 'TODAS') {
          platforms.push(this.publisherPlatform);
        }

        const payload = {
          search_terms: this.searchTerms.trim(),
          limit: parseInt(this.limit, 10) || 25,
          ad_delivery_date_min: this.adDeliveryDateMin || '',
          publisher_platforms: platforms,
          only_whatsapp: this.onlyWhatsApp,
          only_email: this.onlyEmail,
          min_score: parseInt(this.minScore, 10) || 0
        };

        const res = await fetch('/api/search', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });

        const data = await res.json();

        if (!res.ok || data.error) {
          throw new Error(data.message || 'Falha ao processar mineração de leads');
        }

        this.showToast(data.message || `${data.total} leads encontrados e qualificados!`, 'success');
        await this.loadStats();
        await this.loadLeads();
      } catch (err) {
        console.error('Erro durante a busca:', err);
        this.showToast(err.message, 'error');
      } finally {
        clearTimeout(stepTimer1);
        clearTimeout(stepTimer2);
        this.loading = false;
      }
    },

    setTab(tab) {
      this.currentTab = tab;
      this.loadLeads();
    },

    toggleExpand(id) {
      this.expandedLeadId = this.expandedLeadId === id ? null : id;
    },

    async updateLeadStatus(id, newStatus) {
      try {
        const res = await fetch(`/api/leads/${id}/status`, {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ status: newStatus })
        });

        if (!res.ok) {
          throw new Error('Falha ao atualizar status');
        }

        // Se estiver em uma aba específica (ex: 'Novo'), remove imediatamente da visualização atual
        if (this.currentTab !== 'Todos' && this.currentTab !== newStatus) {
          this.leads = this.leads.filter(l => l.id !== id);
          this.totalCount = Math.max(0, this.totalCount - 1);
        } else {
          const index = this.leads.findIndex(l => l.id === id);
          if (index !== -1) {
            this.leads[index].status = newStatus;
          }
        }

        this.showToast(`Lead marcado como "${newStatus}"`, 'success');
        await this.loadStats();
        this.refreshIcons();
      } catch (err) {
        this.showToast(err.message, 'error');
      }
    },

    async deleteLead(id, companyName) {
      const name = companyName || 'este lead';
      if (!confirm(`Deseja realmente excluir "${name}" da base de dados?`)) {
        return;
      }

      try {
        const res = await fetch(`/api/leads/${id}`, {
          method: 'DELETE'
        });

        if (!res.ok) {
          throw new Error('Falha ao excluir lead');
        }

        this.leads = this.leads.filter(l => l.id !== id);
        this.totalCount = Math.max(0, this.totalCount - 1);
        if (this.selectedLeadForCopy && this.selectedLeadForCopy.id === id) {
          this.closeCopyModal();
        }
        this.showToast(`Lead "${name}" excluído com sucesso!`, 'info');
        await this.loadStats();
        this.refreshIcons();
      } catch (err) {
        this.showToast(err.message, 'error');
      }
    },

    copyText(text, successMsg = 'Copiado para a área de transferência!') {
      if (!text) {
        this.showToast('Nada para copiar.', 'info');
        return;
      }
      if (navigator.clipboard && window.isSecureContext) {
        navigator.clipboard.writeText(text).then(() => {
          this.showToast(successMsg, 'success');
        }).catch(() => {
          this.fallbackCopyText(text, successMsg);
        });
      } else {
        this.fallbackCopyText(text, successMsg);
      }
    },

    fallbackCopyText(text, successMsg) {
      try {
        const textArea = document.createElement('textarea');
        textArea.value = text;
        textArea.style.position = 'fixed';
        textArea.style.left = '-999999px';
        textArea.style.top = '-999999px';
        document.body.appendChild(textArea);
        textArea.focus();
        textArea.select();
        const successful = document.execCommand('copy');
        document.body.removeChild(textArea);
        if (successful) {
          this.showToast(successMsg, 'success');
        } else {
          this.showToast('Erro ao copiar texto automaticamente.', 'error');
        }
      } catch (err) {
        this.showToast('Erro ao copiar: ' + err.message, 'error');
      }
    },

    getWhatsAppLink(lead) {
      if (!lead || !lead.extracted_whatsapp) return '';
      let cleanPhone = lead.extracted_whatsapp.replace(/\D/g, '');
      if (!cleanPhone.startsWith('55') && (cleanPhone.length === 10 || cleanPhone.length === 11)) {
        cleanPhone = '55' + cleanPhone;
      }
      const fullMsg = this.getFullMessage(lead);
      return `https://wa.me/${cleanPhone}?text=${encodeURIComponent(fullMsg)}`;
    },

    copyWhatsAppLink(lead) {
      const link = this.getWhatsAppLink(lead);
      if (!link) {
        this.showToast('WhatsApp não disponível para este lead.', 'error');
        return;
      }
      this.copyText(link, 'Link do WhatsApp copiado com sucesso!');
    },

    openWhatsApp(lead) {
      const url = this.getWhatsAppLink(lead);
      if (!url) {
        this.showToast('WhatsApp não disponível para este lead.', 'error');
        return;
      }
      window.open(url, '_blank');
    },

    openEmail(lead) {
      if (!lead || !lead.extracted_email) {
        this.showToast('E-mail não disponível para este lead.', 'error');
        return;
      }
      const subject = encodeURIComponent(`Oportunidade & Parceria - ${lead.company_name}`);
      const body = encodeURIComponent(this.getFullMessage(lead));
      const mailtoURL = `mailto:${lead.extracted_email}?subject=${subject}&body=${body}`;
      
      window.location.href = mailtoURL;
      this.showToast(`Abrindo cliente de e-mail para ${lead.extracted_email}...`, 'info');
    },

    openWebsite(lead) {
      if (!lead) return;
      let url = (lead.landing_page_url || '').trim();
      if (url && (url.startsWith('http://') || url.startsWith('https://'))) {
        window.open(url, '_blank');
        return;
      }
      if (url && url.includes('.')) {
        window.open(`https://${url}`, '_blank');
        return;
      }
      // Fallback: Busca da empresa no Google
      window.open(`https://www.google.com/search?q=${encodeURIComponent(lead.company_name)}`, '_blank');
    },

    exportCSV() {
      window.open('/api/export/csv', '_blank');
    },

    showToast(message, type = 'success') {
      this.toast.message = message;
      this.toast.type = type;
      this.toast.show = true;
      setTimeout(() => {
        this.toast.show = false;
      }, 4000);
    },

    formatDate(dateStr) {
      if (!dateStr) return '';
      try {
        const d = new Date(dateStr);
        return d.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' });
      } catch {
        return dateStr;
      }
    },

    formatPhone(phone) {
      if (!phone) return '';
      const digits = phone.replace(/\D/g, '');
      if (digits.length === 13 && digits.startsWith('55')) {
        return `+55 (${digits.slice(2, 4)}) ${digits.slice(4, 9)}-${digits.slice(9)}`;
      }
      if (digits.length === 12 && digits.startsWith('55')) {
        return `+55 (${digits.slice(2, 4)}) ${digits.slice(4, 8)}-${digits.slice(8)}`;
      }
      if (digits.length === 11) {
        return `(${digits.slice(0, 2)}) ${digits.slice(2, 7)}-${digits.slice(7)}`;
      }
      return phone;
    },

    formatDaysRunning(days) {
      if (typeof days !== 'number' || days <= 0) return 'Ativo hoje';
      if (days === 1) return '1 dia ativo';
      return `${days} dias ativo`;
    },

    formatAdStartDate(dateStr) {
      if (!dateStr) return '';
      try {
        const d = new Date(dateStr);
        if (isNaN(d.getTime())) return dateStr;
        return d.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit', year: 'numeric' });
      } catch {
        return dateStr;
      }
    }
  }));
});
