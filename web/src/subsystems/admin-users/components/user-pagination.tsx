import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Pagination, PaginationContent, PaginationItem } from "@/components/ui/pagination";

export interface UserPaginationProps {
  currentPage: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
}

export function UserPagination({ currentPage, pageSize, total, onPageChange }: UserPaginationProps) {
  const { t } = useTranslation();
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  if (totalPages <= 1) {
    return null;
  }

  return (
    <div className="flex flex-col gap-3 border-t border-border pt-4 sm:flex-row sm:items-center sm:justify-between">
      <p className="text-sm text-muted-foreground">{t("admin_users_page_summary", { page: currentPage, pages: totalPages })}</p>
      <Pagination className="mx-0 w-auto justify-start sm:justify-end">
        <PaginationContent>
          <PaginationItem>
            <Button
              aria-label={t("admin_users_pagination_previous")}
              disabled={currentPage <= 1}
              size="sm"
              variant="outline"
              onClick={() => onPageChange(currentPage - 1)}
            >
              {t("admin_users_pagination_previous")}
            </Button>
          </PaginationItem>
          <PaginationItem>
            <Button
              aria-label={t("admin_users_pagination_next")}
              disabled={currentPage >= totalPages}
              size="sm"
              variant="outline"
              onClick={() => onPageChange(currentPage + 1)}
            >
              {t("admin_users_pagination_next")}
            </Button>
          </PaginationItem>
        </PaginationContent>
      </Pagination>
    </div>
  );
}
