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
        }
      } catch (err) {
        console.error('Erro ao carregar leads:', err);
        this.showToast('Erro ao listar leads do banco de dados', 'error');
      }
    },

    async startSearch() {
      if (!this.searchTerms.trim()) {
        this.showToast('Por favor, digite um nicho ou palavra-chave para buscar.', 'error');
        return;
      }

      this.loading = true;
      this.loadingStep = 'Consultando Meta Ad Library API...';

      const stepTimer1 = setTimeout(() => {
        if (this.loading) this.loadingStep = 'Raspando Landing Pages concorrentemente (WhatsApp, E-mails)...';
      }, 3000);

      const stepTimer2 = setTimeout(() => {
        if (this.loading) this.loadingStep = 'Qualificando maturidade com Google Gemini AI...';
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

        const index = this.leads.findIndex(l => l.id === id);
        if (index !== -1) {
          this.leads[index].status = newStatus;
        }

        this.showToast(`Lead marcado como "${newStatus}"`, 'success');
        this.loadStats();
      } catch (err) {
        this.showToast(err.message, 'error');
      }
    },

    async deleteLead(id) {
      if (!confirm('Deseja realmente excluir este lead da base de dados?')) {
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
        this.showToast('Lead excluído com sucesso', 'info');
        this.loadStats();
      } catch (err) {
        this.showToast(err.message, 'error');
      }
    },

    copyText(text, successMsg = 'Copiado para a área de transferência!') {
      if (!text) {
        this.showToast('Nada para copiar.', 'info');
        return;
      }
      navigator.clipboard.writeText(text).then(() => {
        this.showToast(successMsg, 'success');
      }).catch(() => {
        this.showToast('Erro ao copiar para a área de transferência', 'error');
      });
    },

    openWhatsApp(phone, companyName, icebreaker) {
      if (!phone) {
        this.showToast('WhatsApp não disponível para este lead.', 'error');
        return;
      }
      const cleanPhone = phone.replace(/\D/g, '');
      const defaultMsg = icebreaker ? encodeURIComponent(icebreaker) : encodeURIComponent(`Olá, equipe da ${companyName}!`);
      const url = `https://wa.me/${cleanPhone}?text=${defaultMsg}`;
      window.open(url, '_blank');
    },

    openEmail(email, companyName, icebreaker) {
      if (!email) {
        this.showToast('E-mail não disponível para este lead.', 'error');
        return;
      }
      const subject = encodeURIComponent(`Parceria e Oportunidades - ${companyName}`);
      const body = encodeURIComponent(icebreaker || `Olá equipe da ${companyName},\n\n`);
      const mailtoURL = `mailto:${email}?subject=${subject}&body=${body}`;
      
      window.location.href = mailtoURL;
      this.showToast(`Abrindo cliente de e-mail para ${email}...`, 'info');
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
    }
  }));
});
